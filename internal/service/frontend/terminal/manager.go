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
	// ErrReservationRequired indicates that a session registration was attempted without reserving a slot.
	ErrReservationRequired = errors.New("terminal session reservation required")
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
func (m *Manager) Acquire() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shuttingDown {
		return ErrManagerShuttingDown
	}
	if m.reservedSlots+len(m.sessions) >= m.maxSessions {
		return ErrMaxSessionsReached
	}
	m.reservedSlots++
	return nil
}

// ReleasePending releases a slot that was reserved but never turned into an active session.
func (m *Manager) ReleasePending() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.reservedSlots > 0 {
		m.reservedSlots--
	}
}

// Register marks a reserved slot as an active terminal session.
func (m *Manager) Register(conn *Connection) error {
	if conn == nil {
		return errors.New("nil terminal connection")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.reservedSlots <= 0 {
		return ErrReservationRequired
	}
	m.reservedSlots--

	if m.shuttingDown {
		return ErrManagerShuttingDown
	}

	m.sessions[conn.ID] = conn
	m.wg.Add(1)
	return nil
}

// ReleaseSession unregisters an active session and frees its slot.
func (m *Manager) ReleaseSession(id string) {
	m.mu.Lock()
	_, ok := m.sessions[id]
	if ok {
		delete(m.sessions, id)
	}
	m.mu.Unlock()

	if ok {
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
