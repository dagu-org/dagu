// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package publicwebhook

import (
	"cmp"
	"context"
	"errors"
	"math/rand/v2"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/dagucloud/dagu/v2/internal/auth"
	"github.com/dagucloud/dagu/v2/internal/cloudrelay"
	"github.com/dagucloud/dagu/v2/internal/cmn/logger"
	"github.com/dagucloud/dagu/v2/internal/cmn/logger/tag"
	"github.com/dagucloud/dagu/v2/internal/intake"
	"github.com/dagucloud/dagu/v2/internal/ir"
	"github.com/dagucloud/dagu/v2/internal/license"
	"github.com/dagucloud/dagu/v2/internal/webhooksig"
)

const (
	// maxQueued is how many runs may wait on a DAG's queue before its
	// deliveries wait at the relay instead.
	maxQueued = 100
	// queueFullRetry and storageRetry are how long a delivery waits at the
	// relay when the queue is full, or when queueing it failed.
	queueFullRetry = time.Minute
	storageRetry   = 30 * time.Second
	// hookSuffix ends every hook ID: auth.WebhookHookID.
	hookSuffix = "/webhook"
	// requestTimeout bounds a ticket request and the relay handshake.
	requestTimeout = 30 * time.Second
)

// Status errors the page shows when the server holds no public URLs.
const (
	statusNotConnected = "This server is not connected to Dagu Cloud."
	statusUnreadable   = "The webhooks could not be read."
	statusTooManyHooks = "Dagu Cloud refused more public URLs than the plan allows."
)

// Webhooks reads the webhooks that may have public URLs.
type Webhooks interface {
	// ListComplete returns every webhook, or an error when any cannot be
	// read.
	ListComplete(ctx context.Context) ([]*auth.Webhook, error)
	GetByDAGName(ctx context.Context, dagName string) (*auth.Webhook, error)
}

// Runs starts the runs deliveries ask for.
type Runs interface {
	// DAG loads the DAG a webhook starts.
	DAG(ctx context.Context, dagName string) (*ir.DAG, error)
	// Queued counts the runs waiting on the DAG's queue.
	Queued(ctx context.Context, dag *ir.DAG) (int, error)
	// Enqueue queues a run of the DAG named runID with params. A run by that
	// ID that already exists counts as queued. An error wrapping
	// ErrInvalidRun means waiting cannot make the run start.
	Enqueue(ctx context.Context, dagName string, dag *ir.DAG, runID, params string) error
}

// ErrInvalidRun marks a run the DAG or the delivery cannot start.
var ErrInvalidRun = errors.New("the run cannot start")

// Config is what the service works with.
type Config struct {
	// Credentials returns the server's Dagu Cloud credentials, and false
	// while it is not connected.
	Credentials func() (license.CloudCredentials, bool)
	Webhooks    Webhooks
	Store       Store
	Runs        Runs
	// HTTPClient reaches Dagu Cloud; nil means a client with a 30-second
	// timeout.
	HTTPClient *http.Client
	Hello      cloudrelay.Hello
}

// timing is how the service paces itself; tests shorten it.
type timing struct {
	// check is how often the webhooks are read again for changes to the
	// relay's set, and how often an idle service looks for a reason to
	// connect.
	check time.Duration
	// minBackoff and maxBackoff bound the wait before connecting again
	// after a connection failed.
	minBackoff time.Duration
	maxBackoff time.Duration
	// stable is how long a connection must have lasted for a failure after
	// it to start the backoff afresh.
	stable time.Duration
	// refused is how long the service waits after Dagu Cloud refused a
	// ticket, which asking again soon does not change.
	refused time.Duration
}

var defaultTiming = timing{
	check:      10 * time.Second,
	minBackoff: time.Second,
	maxBackoff: time.Minute,
	stable:     time.Minute,
	refused:    10 * time.Minute,
}

// Service holds the relay connection for the server's public webhook URLs.
type Service struct {
	cfg    Config
	timing timing

	mu sync.Mutex
	// status is the relay's status as last stored.
	status Status
}

// New creates a Service.
func New(cfg Config) *Service {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: requestTimeout}
	}
	return &Service{cfg: cfg, timing: defaultTiming}
}

// Run holds the relay connection while the server is connected to Dagu
// Cloud and some webhook has a public URL, until ctx is done.
func (s *Service) Run(ctx context.Context) {
	backoff := s.timing.minBackoff
	for {
		wait := s.timing.check
		creds, connected := s.cfg.Credentials()
		webhooks, err := s.cfg.Webhooks.ListComplete(ctx)
		switch {
		case ctx.Err() != nil:
			return
		case !connected:
			s.setStatus(ctx, Status{Error: statusNotConnected})
		case err != nil:
			logger.Warn(ctx, "Public webhooks: the webhooks could not be read", tag.Error(err))
			s.setError(ctx, statusUnreadable)
		case !slices.ContainsFunc(webhooks, isPublic):
			s.setStatus(ctx, Status{})
		default:
			started := time.Now()
			err := s.connect(ctx, creds)
			if ctx.Err() != nil {
				return
			}
			_, refused := errors.AsType[*cloudrelay.TicketRefusedError](err)
			switch {
			case err == nil:
				// The relay ended the connection on schedule, or no URL is
				// public any longer.
				backoff = s.timing.minBackoff
				wait = 0
			case refused:
				wait = s.timing.refused
				s.setError(ctx, err.Error())
			default:
				if time.Since(started) >= s.timing.stable {
					backoff = s.timing.minBackoff
				}
				wait = backoff/2 + rand.N(backoff/2+1) //nolint:gosec // jitter, not cryptographic
				backoff = min(2*backoff, s.timing.maxBackoff)
				logger.Warn(ctx, "Public webhooks: the relay connection failed", tag.Error(err))
				s.setError(ctx, err.Error())
			}
		}
		if !sleep(ctx, wait) {
			return
		}
	}
}

// connect holds one relay connection, sending the relay the set of public
// URLs whenever it changes, until the connection ends or no URL is public.
func (s *Service) connect(ctx context.Context, creds license.CloudCredentials) error {
	ticket, err := cloudrelay.RequestTicket(ctx, s.cfg.HTTPClient, creds)
	if err != nil {
		return err
	}
	conn, err := cloudrelay.Dial(ctx, s.cfg.HTTPClient, ticket, s.cfg.Hello)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	logger.Info(ctx, "Public webhooks: connected to the relay")
	s.updateStatus(ctx, func(st *Status) {
		st.Connected = true
		st.MaxHooks = ticket.MaxHooks
		st.Error = ""
	})
	defer s.updateStatus(context.WithoutCancel(ctx), func(st *Status) { st.Connected = false })

	served := make(chan error, 1)
	go func() {
		served <- conn.Serve(ctx, cloudrelay.Handler{
			Delivery: s.deliver,
			Hooked:   func(h cloudrelay.Hooked) { s.hooked(ctx, h) },
			Missed:   func(m []cloudrelay.Missed) { s.missed(ctx, m) },
		})
	}()

	var sent string
	for {
		// A new set drops what waits for any hook it leaves out, so it is
		// sent only from a complete read; after a failed one the last set
		// stays.
		webhooks, err := s.cfg.Webhooks.ListComplete(ctx)
		switch {
		case err != nil:
			logger.Warn(ctx, "Public webhooks: the webhooks could not be read; the relay keeps its set", tag.Error(err))
		default:
			hooks, leftOut := hookSet(webhooks, ticket.MaxHooks)
			if signature := setSignature(hooks); signature != sent {
				if err := conn.SetHooks(ctx, hooks); err != nil {
					break
				}
				sent = signature
				s.updateStatus(ctx, func(st *Status) { st.LeftOut = leftOut })
			}
			if len(hooks) == 0 {
				cancel()
				<-served
				return nil
			}
		}
		select {
		case err := <-served:
			return err
		case <-time.After(s.timing.check):
		}
	}
}

// isPublic reports whether a webhook has a public URL.
func isPublic(wh *auth.Webhook) bool {
	return wh.Public != nil && wh.Public.Token != ""
}

// hookSet is the set of public URLs the relay holds: at most maxHooks, the
// oldest first. It also returns the DAGs left out.
func hookSet(webhooks []*auth.Webhook, maxHooks int) ([]cloudrelay.Hook, []string) {
	public := slices.DeleteFunc(slices.Clone(webhooks), func(wh *auth.Webhook) bool { return !isPublic(wh) })
	slices.SortFunc(public, func(a, b *auth.Webhook) int {
		return cmp.Or(a.Public.CreatedAt.Compare(b.Public.CreatedAt), strings.Compare(a.DAGName, b.DAGName))
	})
	hooks := make([]cloudrelay.Hook, 0, len(public))
	var leftOut []string
	for _, wh := range public {
		if len(hooks) >= maxHooks {
			leftOut = append(leftOut, wh.DAGName)
			continue
		}
		hook := cloudrelay.Hook{ID: auth.WebhookHookID(wh.DAGName), Token: wh.Public.Token}
		if wh.Public.Sender == string(webhooksig.Slack) {
			hook.Handshake = cloudrelay.HandshakeSlack
		}
		hooks = append(hooks, hook)
	}
	return hooks, leftOut
}

// setSignature tells sets of hooks apart without keeping their tokens.
func setSignature(hooks []cloudrelay.Hook) string {
	var b strings.Builder
	for _, h := range hooks {
		b.WriteString(h.ID)
		b.WriteByte(0)
		b.WriteString(h.Token)
		b.WriteByte(0)
		b.WriteString(h.Handshake)
		b.WriteByte('\n')
	}
	return b.String()
}

func (s *Service) hooked(ctx context.Context, h cloudrelay.Hooked) {
	s.updateStatus(ctx, func(st *Status) {
		if h.Error == cloudrelay.ErrorTooManyHooks {
			st.Error = statusTooManyHooks
			return
		}
		if h.Error != "" {
			st.Error = h.Error
			return
		}
		st.URL = h.URL
		st.Error = ""
	})
}

// missed counts what the relay dropped for each public webhook. The relay
// reports counts as soon as the server says hello, before it hears the set,
// so each hook is looked up in the store rather than in the set.
func (s *Service) missed(ctx context.Context, missed []cloudrelay.Missed) {
	now := time.Now().UTC()
	for _, m := range missed {
		dagName, ok := strings.CutSuffix(m.Hook, hookSuffix)
		if !ok {
			continue
		}
		wh, err := s.cfg.Webhooks.GetByDAGName(ctx, dagName)
		if err != nil || !isPublic(wh) {
			if err != nil && !errors.Is(err, auth.ErrWebhookNotFound) {
				logger.Warn(ctx, "Public webhooks: missed deliveries could not be counted", tag.DAG(dagName), tag.Error(err))
			}
			continue
		}
		if err := s.cfg.Store.AddMissed(ctx, wh.ID, m.Expired, m.Refused, now); err != nil {
			logger.Warn(ctx, "Public webhooks: missed deliveries could not be counted", tag.DAG(dagName), tag.Error(err))
		}
	}
}

// deliver starts a run for a delivery the webhook takes, and logs what
// became of it.
func (s *Service) deliver(ctx context.Context, d cloudrelay.Delivery) cloudrelay.Outcome {
	dagName, ok := strings.CutSuffix(d.Hook, hookSuffix)
	if !ok {
		return cloudrelay.OutcomeIgnored
	}
	wh, err := s.cfg.Webhooks.GetByDAGName(ctx, dagName)
	switch {
	case errors.Is(err, auth.ErrWebhookNotFound):
		return cloudrelay.OutcomeIgnored
	case err != nil:
		logger.Warn(ctx, "Public webhooks: a webhook could not be read", tag.DAG(dagName), tag.Error(err))
		return cloudrelay.OutcomeRetry(storageRetry)
	case !isPublic(wh):
		return cloudrelay.OutcomeIgnored
	}

	event := webhooksig.Event(d.Header, d.Body)
	logged := Delivery{ID: d.ID, Received: d.Received.UTC(), Size: len(d.Body), Event: event}
	record := func(outcome, runID, reason string) {
		logged.Outcome, logged.RunID, logged.Error = outcome, runID, reason
		if err := s.cfg.Store.AddDelivery(ctx, wh.ID, logged); err != nil {
			logger.Warn(ctx, "Public webhooks: a delivery could not be logged", tag.DAG(dagName), tag.Error(err))
		}
	}

	if !wh.Enabled {
		record(OutcomeIgnored, "", "The webhook is turned off.")
		return cloudrelay.OutcomeIgnored
	}
	if wh.Public.Sender != "" {
		if err := webhooksig.Verify(webhooksig.Sender(wh.Public.Sender), wh.Public.SenderSecret, d.Header, d.Body, d.Received); err != nil {
			record(OutcomeRejected, "", err.Error())
			return cloudrelay.OutcomeIgnored
		}
	}
	switch {
	case webhooksig.IsGitHubPing(d.Header):
		record(OutcomeSkipped, "", "GitHub's ping starts no run.")
		return cloudrelay.OutcomeIgnored
	case !webhooksig.Takes(wh.Public.Events, event):
		reason := "The webhook does not take this event."
		if event == "" {
			reason = "The request names no event, and the webhook takes only named events."
		}
		record(OutcomeSkipped, "", reason)
		return cloudrelay.OutcomeIgnored
	}

	dag, err := s.cfg.Runs.DAG(ctx, dagName)
	if err != nil {
		record(OutcomeFailed, "", err.Error())
		return cloudrelay.OutcomeFailed
	}
	queued, err := s.cfg.Runs.Queued(ctx, dag)
	if err != nil {
		record(OutcomeWaiting, "", err.Error())
		return cloudrelay.OutcomeRetry(storageRetry)
	}
	if queued >= maxQueued {
		record(OutcomeWaiting, "", "100 runs are already queued; the delivery waits at Dagu Cloud.")
		return cloudrelay.OutcomeRetry(queueFullRetry)
	}
	var forward []string
	if dag.Webhook != nil {
		forward = dag.Webhook.ForwardHeaders
	}
	headers, err := intake.WebhookHeaders(forward, d.Header.Values)
	if err != nil {
		record(OutcomeFailed, "", err.Error())
		return cloudrelay.OutcomeFailed
	}
	err = s.cfg.Runs.Enqueue(ctx, dagName, dag, d.ID, intake.WebhookRuntimeParams(string(d.Body), headers))
	switch {
	case errors.Is(err, ErrInvalidRun):
		record(OutcomeFailed, "", err.Error())
		return cloudrelay.OutcomeFailed
	case err != nil:
		record(OutcomeWaiting, "", err.Error())
		return cloudrelay.OutcomeRetry(storageRetry)
	}
	record(OutcomeStarted, d.ID, "")
	return cloudrelay.OutcomeStarted
}

// setStatus stores status, as the relay connection now is, when it differs
// from what was last stored.
func (s *Service) setStatus(ctx context.Context, status Status) {
	s.updateStatus(ctx, func(st *Status) { *st = status })
}

// setError records why the server holds no relay connection, keeping the URL
// the relay last gave, which stays valid while it reconnects.
func (s *Service) setError(ctx context.Context, reason string) {
	s.updateStatus(ctx, func(st *Status) {
		st.Connected = false
		st.Error = reason
	})
}

// updateStatus applies change to the relay's status and stores the result
// when it differs from what was last stored.
func (s *Service) updateStatus(ctx context.Context, change func(*Status)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.status
	next.LeftOut = slices.Clone(next.LeftOut)
	change(&next)
	next.UpdatedAt = s.status.UpdatedAt
	if equalStatus(next, s.status) {
		return
	}
	next.UpdatedAt = time.Now().UTC()
	if err := s.cfg.Store.SetStatus(ctx, next); err != nil {
		logger.Warn(ctx, "Public webhooks: the relay status could not be stored", tag.Error(err))
		return
	}
	s.status = next
}

func equalStatus(a, b Status) bool {
	return a.Connected == b.Connected && a.URL == b.URL && a.MaxHooks == b.MaxHooks &&
		a.Error == b.Error && slices.Equal(a.LeftOut, b.LeftOut) && a.UpdatedAt.Equal(b.UpdatedAt)
}

// sleep waits for d, and reports false when ctx ended first.
func sleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
