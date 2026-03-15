// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package terminal

import (
	"context"
	"errors"
	"sync"
)

var (
	// ErrMaxSessionsReached indicates that the terminal session limit has been reached.
	ErrMaxSessionsReached = errors.New("max terminal sessions reached")
	// ErrManagerShuttingDown indicates that the terminal manager is shutting down.
	ErrManagerShuttingDown = errors.New("terminal manager is shutting down")
	// ErrReservationInactive indicates that a released or already-activated lease was reused.
	ErrReservationInactive = errors.New("terminal session reservation is inactive")
)

// Manager tracks active terminal sessions and coordinates server shutdown.
type Manager struct {
	ctx    context.Context
	cancel context.CancelFunc

	maxSessions int

	mu            sync.Mutex
	reservedSlots int
	shuttingDown  bool
	sessions      map[string]*Connection
	wg            sync.WaitGroup
}

type leaseState uint8

const (
	leaseStateReserved leaseState = iota + 1
	leaseStateActive
	leaseStateReleased
)

// sessionLease represents a reserved terminal slot that can be activated once.
type sessionLease struct {
	manager *Manager
	state   leaseState
	connID  string
}

// NewManager creates a terminal session manager bound to the server lifetime.
func NewManager(parent context.Context, maxSessions int) *Manager {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	return &Manager{
		ctx:         ctx,
		cancel:      cancel,
		maxSessions: maxSessions,
		sessions:    make(map[string]*Connection),
	}
}

// Context returns the manager context used to shut down active terminal sessions.
func (m *Manager) Context() context.Context {
	return m.ctx
}

// Acquire reserves capacity for a new terminal session before the WebSocket upgrade.
func (m *Manager) Acquire() (*sessionLease, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shuttingDown {
		return nil, ErrManagerShuttingDown
	}
	if m.reservedSlots+len(m.sessions) >= m.maxSessions {
		return nil, ErrMaxSessionsReached
	}
	m.reservedSlots++
	return &sessionLease{
		manager: m,
		state:   leaseStateReserved,
	}, nil
}

// Activate turns a reserved lease into an active terminal session.
func (l *sessionLease) Activate(conn *Connection) error {
	if conn == nil {
		return errors.New("nil terminal connection")
	}
	if l == nil || l.manager == nil {
		return ErrReservationInactive
	}

	m := l.manager
	m.mu.Lock()
	defer m.mu.Unlock()

	if l.state != leaseStateReserved {
		return ErrReservationInactive
	}
	if m.reservedSlots <= 0 {
		l.state = leaseStateReleased
		return ErrReservationInactive
	}
	m.reservedSlots--

	if m.shuttingDown {
		l.state = leaseStateReleased
		return ErrManagerShuttingDown
	}

	m.sessions[conn.ID] = conn
	m.wg.Add(1)
	l.connID = conn.ID
	l.state = leaseStateActive
	return nil
}

// Release releases a reserved or active lease exactly once.
func (l *sessionLease) Release() {
	if l == nil || l.manager == nil {
		return
	}

	m := l.manager
	m.mu.Lock()
	var doneActive bool
	switch l.state {
	case leaseStateReserved:
		if m.reservedSlots > 0 {
			m.reservedSlots--
		}
		l.state = leaseStateReleased
	case leaseStateActive:
		if _, ok := m.sessions[l.connID]; ok {
			delete(m.sessions, l.connID)
			doneActive = true
		}
		l.state = leaseStateReleased
	case leaseStateReleased:
		// Already released.
	}
	m.mu.Unlock()

	if doneActive {
		m.wg.Done()
	}
}

// Shutdown cancels all active sessions and waits for cleanup to finish.
func (m *Manager) Shutdown(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	m.mu.Lock()
	m.shuttingDown = true
	m.mu.Unlock()

	m.cancel()

	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
	}

	for _, conn := range m.snapshotSessions() {
		conn.ForceKill()
	}

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Manager) snapshotSessions() []*Connection {
	m.mu.Lock()
	defer m.mu.Unlock()

	sessions := make([]*Connection, 0, len(m.sessions))
	for _, conn := range m.sessions {
		sessions = append(sessions, conn)
	}
	return sessions
}
