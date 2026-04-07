// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package automata

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/dagucloud/dagu/internal/agent"
	"github.com/dagucloud/dagu/internal/cmn/config"
	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/core/exec"
	"github.com/dagucloud/dagu/internal/runtime"
	"github.com/dagucloud/dagu/internal/service/coordinator"
	"github.com/dagucloud/dagu/internal/service/eventstore"
)

var automataNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_]*$`)

const (
	definitionFilePerm = 0o600
	stateFilePerm      = 0o600
	dirPerm            = 0o750
)

type Service struct {
	cfg            *config.Config
	definitionsDir string
	stateDir       string
	dagStore       exec.DAGStore
	dagRunStore    exec.DAGRunStore
	dagRunCtrl     dagRunController
	coordinatorCli coordinatorCanceler
	sessionStore   agent.SessionStore
	memoryStore    agent.MemoryStore
	agentAPI       *agent.API
	soulStore      agent.SoulStore
	subCmdBuilder  *runtime.SubCmdBuilder
	eventService   *eventstore.Service
	eventSource    eventstore.Source
	logger         *slog.Logger
	clock          func() time.Time
	reconcileEvery time.Duration
	mu             sync.Mutex
}

type Option func(*Service)

type dagRunController interface {
	Stop(ctx context.Context, dag *core.DAG, dagRunID string) error
}

type coordinatorCanceler interface {
	RequestCancel(ctx context.Context, dagName, dagRunID string, rootRef *exec.DAGRunRef) error
}

type sessionUserReassigner interface {
	ReassignSessionUser(ctx context.Context, sessionID, userID string) error
}

func WithAgentAPI(api *agent.API) Option {
	return func(s *Service) {
		s.agentAPI = api
	}
}

func WithSoulStore(store agent.SoulStore) Option {
	return func(s *Service) {
		s.soulStore = store
	}
}

func WithSessionStore(store agent.SessionStore) Option {
	return func(s *Service) {
		s.sessionStore = store
	}
}

func WithMemoryStore(store agent.MemoryStore) Option {
	return func(s *Service) {
		s.memoryStore = store
	}
}

func WithDAGRunController(ctrl dagRunController) Option {
	return func(s *Service) {
		s.dagRunCtrl = ctrl
	}
}

func WithCoordinatorClient(cli coordinator.Client) Option {
	return func(s *Service) {
		s.coordinatorCli = cli
	}
}

func WithSubCmdBuilder(builder *runtime.SubCmdBuilder) Option {
	return func(s *Service) {
		s.subCmdBuilder = builder
	}
}

func WithEventService(service *eventstore.Service) Option {
	return func(s *Service) {
		s.eventService = service
	}
}

func WithEventSource(source eventstore.Source) Option {
	return func(s *Service) {
		s.eventSource = source
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(s *Service) {
		if logger != nil {
			s.logger = logger
		}
	}
}

func WithClock(clock func() time.Time) Option {
	return func(s *Service) {
		if clock != nil {
			s.clock = clock
		}
	}
}

func New(cfg *config.Config, dagStore exec.DAGStore, dagRunStore exec.DAGRunStore, opts ...Option) *Service {
	svc := &Service{
		cfg:            cfg,
		definitionsDir: filepath.Join(cfg.Paths.DAGsDir, "automata"),
		stateDir:       filepath.Join(cfg.Paths.DataDir, "automata"),
		dagStore:       dagStore,
		dagRunStore:    dagRunStore,
		logger:         slog.Default(),
		clock:          time.Now,
		reconcileEvery: 2 * time.Second,
		eventSource: eventstore.Source{
			Service: eventstore.SourceServiceUnknown,
		},
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

func validateName(name string) error {
	if !automataNamePattern.MatchString(name) {
		return fmt.Errorf("invalid automata name %q", name)
	}
	return nil
}
