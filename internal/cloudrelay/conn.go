// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cloudrelay

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

// Relay frame types, as the relay and the server agree on them.
const (
	frameHello     = "hello"
	frameHooks     = "hooks"
	frameHooked    = "hooked"
	frameDelivery  = "delivery"
	frameDelivered = "delivered"
	frameMissed    = "missed"
)

const (
	relayPing = "ping"
	relayPong = "pong"
	// Why the relay ends a connection: on schedule, every 12 hours; because
	// another connection from the same server took over; or because Dagu
	// Cloud disconnected the server.
	closeExpired  = "expired"
	closeReplaced = "replaced"
	closeDropped  = "dropped"
	// readLimit bounds one frame from the relay: a request body of 512 KiB,
	// escaped as JSON, with its headers.
	readLimit = 8 << 20
)

// timing is how a connection keeps itself known to be alive.
type timing struct {
	// pingInterval spaces the pings. The relay answers each without waking,
	// and takes a socket silent for 45 seconds for offline.
	pingInterval time.Duration
	// silence is how long the relay may say nothing before the connection
	// is taken for dead, as the relay takes the server.
	silence time.Duration
	// pongWait is how long a ping must have been out unanswered first, so a
	// ping held behind a long write is not blamed on the relay.
	pongWait time.Duration
	// watchInterval spaces the checks for an unanswered ping.
	watchInterval time.Duration
	// writeTimeout bounds one write, so a connection the network has
	// silently dropped cannot hold a write forever.
	writeTimeout time.Duration
}

var defaultTiming = timing{
	pingInterval:  15 * time.Second,
	silence:       45 * time.Second,
	pongWait:      30 * time.Second,
	watchInterval: 5 * time.Second,
	writeTimeout:  time.Minute,
}

var (
	// ErrSilent means the relay stopped answering.
	ErrSilent = errors.New("the relay stopped answering")
	// ErrReplaced means another connection from this server took over.
	ErrReplaced = errors.New("another connection from this server took over the relay")
	// ErrDropped means Dagu Cloud disconnected this server.
	ErrDropped = errors.New("the server was disconnected from Dagu Cloud")
)

// Hello is what the server tells the relay first.
type Hello struct {
	Name    string
	Version string
}

// Hook is one public webhook URL as the relay knows it: an ID the server
// chooses, the token its URL ends in, and the sender whose URL check the
// relay answers for it.
type Hook struct {
	ID    string
	Token string
	// Handshake names that sender: HandshakeSlack, or empty for none.
	Handshake string
}

// HandshakeSlack has the relay answer Slack's URL check, which needs its
// challenge back at once, even while the server is away.
const HandshakeSlack = "slack"

// Hooked is the relay's answer to a set of hooks: the URL they share, or why
// it refused the set.
type Hooked struct {
	URL string
	// Error is "too_many_hooks" when the set is larger than the ticket allows.
	Error string
}

// ErrorTooManyHooks is the relay's refusal of a set larger than the ticket
// allows.
const ErrorTooManyHooks = "too_many_hooks"

// Missed counts the webhooks the relay dropped for one hook: those that
// waited past its 7 days, and those it refused while too many waited.
type Missed struct {
	Hook    string `json:"hook"`
	Expired int    `json:"expired"`
	Refused int    `json:"refused"`
}

// Delivery is a webhook the relay kept for one of this server's hooks.
type Delivery struct {
	// ID tells this delivery apart from every other, across repeats.
	ID       string
	Hook     string
	Received time.Time
	Header   http.Header
	Body     []byte
}

// Outcome is what became of a delivery. The relay forgets a delivery on any
// outcome but a retry.
type Outcome struct {
	name string
	// after is how long a retried delivery waits at the relay.
	after time.Duration
}

var (
	OutcomeStarted = Outcome{name: "started"}
	OutcomeIgnored = Outcome{name: "ignored"}
	OutcomeFailed  = Outcome{name: "failed"}
)

// OutcomeRetry asks the relay to keep a delivery and send it again after the
// given time, holding back its hook's later deliveries until then.
func OutcomeRetry(after time.Duration) Outcome {
	return Outcome{name: "retry", after: after}
}

// String is the outcome's name on the wire.
func (o Outcome) String() string { return o.name }

// Handler serves what the relay sends to this server.
type Handler struct {
	// Delivery takes a webhook and reports what became of it. The relay
	// sends a webhook again on the next connection until it hears an
	// outcome, so the same delivery can arrive more than once.
	Delivery func(ctx context.Context, d Delivery) Outcome
	// Hooked receives the relay's answer to each set of hooks.
	Hooked func(h Hooked)
	// Missed receives, for each hook, how many webhooks the relay dropped
	// since it last told this server. Each count is told once.
	Missed func(missed []Missed)
}

// Conn is a connection this server holds open to the relay.
type Conn struct {
	ws     *websocket.Conn
	timing timing
	// reads counts the messages read from the relay, the latest at heard.
	// pingMark is what it was when the oldest unanswered ping left, at
	// pinged. Times are Unix nanoseconds.
	reads, heard, pingMark, pinged atomic.Int64
	// silent records that the watchdog closed the connection.
	silent atomic.Bool
	// hookSets numbers the hook sets sent.
	hookSets atomic.Int64
}

// frame is one JSON message between the relay and the server.
type frame struct {
	Type    string            `json:"t"`
	ID      string            `json:"id,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    json.RawMessage   `json:"body,omitempty"`
	Name    string            `json:"name,omitempty"`
	Version string            `json:"version,omitempty"`
	// Hooks is a pointer so that an empty set is still sent.
	Hooks    *[]relayHook `json:"hooks,omitempty"`
	Hook     string       `json:"hook,omitempty"`
	Received string       `json:"received,omitempty"`
	URL      string       `json:"url,omitempty"`
	Error    string       `json:"error,omitempty"`
	Outcome  string       `json:"outcome,omitempty"`
	// After is how many seconds a retried delivery waits.
	After int `json:"after,omitempty"`
}

// relayHook is a hook as the relay stores it: the token only as a digest.
type relayHook struct {
	Hook      string `json:"hook"`
	Digest    string `json:"digest"`
	Handshake string `json:"handshake,omitempty"`
}

// Dial opens a connection to the relay a ticket names and introduces this
// server.
func Dial(ctx context.Context, client *http.Client, ticket Ticket, hello Hello) (*Conn, error) {
	header := http.Header{"Authorization": {"Bearer " + ticket.Token}, "User-Agent": {userAgent()}}
	ws, res, err := websocket.Dial(ctx, ticket.URL, &websocket.DialOptions{HTTPClient: client, HTTPHeader: header})
	if res != nil && res.Body != nil {
		_ = res.Body.Close()
	}
	if err != nil {
		if res != nil {
			return nil, fmt.Errorf("the relay refused this server (%d)", res.StatusCode)
		}
		return nil, fmt.Errorf("connect to the relay: %w", err)
	}
	ws.SetReadLimit(readLimit)
	conn := &Conn{ws: ws, timing: defaultTiming}
	conn.heard.Store(time.Now().UnixNano())
	if err := conn.send(ctx, frame{Type: frameHello, Name: hello.Name, Version: hello.Version}); err != nil {
		_ = ws.CloseNow()
		return nil, err
	}
	return conn, nil
}

// Close ends the connection without serving it.
func (c *Conn) Close() error { return c.ws.CloseNow() }

// SetHooks gives the relay the whole set of this server's hooks, replacing
// the set it holds. Webhooks waiting for a hook left out are dropped. The
// relay answers through Handler.Hooked.
func (c *Conn) SetHooks(ctx context.Context, hooks []Hook) error {
	set := make([]relayHook, 0, len(hooks))
	for _, h := range hooks {
		digest := sha256.Sum256([]byte(h.Token))
		set = append(set, relayHook{Hook: h.ID, Digest: base64.RawURLEncoding.EncodeToString(digest[:]), Handshake: h.Handshake})
	}
	return c.send(ctx, frame{Type: frameHooks, ID: strconv.FormatInt(c.hookSets.Add(1), 10), Hooks: &set})
}

// Serve answers what the relay sends until the connection ends or ctx is
// done. Deliveries run concurrently. It returns nil when the relay ended the
// connection on schedule.
func (c *Conn) Serve(ctx context.Context, h Handler) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer func() { _ = c.ws.CloseNow() }()
	go c.ping(ctx)
	go c.watch(ctx)
	for {
		kind, data, err := c.ws.Read(ctx)
		if err != nil {
			if c.silent.Load() {
				return ErrSilent
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return closed(err)
		}
		c.heard.Store(time.Now().UnixNano())
		c.reads.Add(1)
		var f frame
		if kind != websocket.MessageText || string(data) == relayPong || json.Unmarshal(data, &f) != nil {
			continue
		}
		switch f.Type {
		case frameDelivery:
			go func() {
				outcome := deliver(ctx, h.Delivery, f)
				// A delivery cut short by the connection ending is sent again
				// on the next one, so it gets no outcome here.
				if ctx.Err() == nil {
					after := int(math.Ceil(outcome.after.Seconds()))
					_ = c.send(ctx, frame{Type: frameDelivered, ID: f.ID, Outcome: outcome.String(), After: after})
				}
			}()
		case frameHooked:
			if h.Hooked != nil {
				h.Hooked(Hooked{URL: f.URL, Error: f.Error})
			}
		case frameMissed:
			var missed struct {
				Hooks []Missed `json:"hooks"`
			}
			if h.Missed != nil && json.Unmarshal(data, &missed) == nil && len(missed.Hooks) > 0 {
				h.Missed(missed.Hooks)
			}
		}
	}
}

// closed explains why the connection ended, or is nil when the relay ended
// it on schedule.
func closed(err error) error {
	closeErr, ok := errors.AsType[websocket.CloseError](err)
	switch {
	case !ok:
		return err
	case closeErr.Code == websocket.StatusNormalClosure && closeErr.Reason == closeExpired:
		return nil
	case closeErr.Reason == closeReplaced:
		return ErrReplaced
	case closeErr.Reason == closeDropped:
		return ErrDropped
	}
	return fmt.Errorf("the relay closed the connection (%d %s)", closeErr.Code, closeErr.Reason)
}

// ping pings the relay, recording when the oldest unanswered ping actually
// left. A ping waiting behind a long write is not counted until it leaves.
func (c *Conn) ping(ctx context.Context) {
	ticker := time.NewTicker(c.timing.pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		reads := c.reads.Load()
		answered := c.pinged.Load() == 0 || reads != c.pingMark.Load()
		if c.write(ctx, []byte(relayPing)) == nil && answered {
			c.pingMark.Store(reads)
			c.pinged.Store(time.Now().UnixNano())
		}
	}
}

// watch closes a connection the relay has said nothing on for silence, with
// a ping out unanswered for pongWait, which a network change or sleep can
// leave open on this side alone.
func (c *Conn) watch(ctx context.Context) {
	ticker := time.NewTicker(c.timing.watchInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		pinged := c.pinged.Load()
		if pinged != 0 && c.reads.Load() == c.pingMark.Load() && time.Since(time.Unix(0, pinged)) > c.timing.pongWait &&
			time.Since(time.Unix(0, c.heard.Load())) > c.timing.silence {
			c.silent.Store(true)
			_ = c.ws.CloseNow()
			return
		}
	}
}

func (c *Conn) send(ctx context.Context, f frame) error {
	data, err := encode(f)
	if err != nil {
		return err
	}
	return c.write(ctx, data)
}

// write sends one message, giving up after the write timeout. A write that
// stops part way closes the connection.
func (c *Conn) write(ctx context.Context, data []byte) error {
	ctx, cancel := context.WithTimeout(ctx, c.timing.writeTimeout)
	defer cancel()
	return c.ws.Write(ctx, websocket.MessageText, data)
}

// encode encodes a frame as the relay reads it. HTML is not escaped, which
// would let a body grow sixfold.
func encode(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

// deliver hands a delivery frame to take. One that cannot be read, or that
// nothing takes, is ignored.
func deliver(ctx context.Context, take func(context.Context, Delivery) Outcome, f frame) Outcome {
	var body string
	received, err := time.Parse(time.RFC3339Nano, f.Received)
	if take == nil || err != nil || json.Unmarshal(f.Body, &body) != nil {
		return OutcomeIgnored
	}
	header := http.Header{}
	for name, value := range f.Headers {
		header.Set(name, value)
	}
	return take(ctx, Delivery{ID: f.ID, Hook: f.Hook, Received: received, Header: header, Body: []byte(body)})
}
