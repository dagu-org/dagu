// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package terminal

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	// ErrMaxSessionsReached indicates that the terminal session limit has been reached.
	ErrMaxSessionsReached = errors.New("max terminal sessions reached")
	// ErrManagerShuttingDown indicates that the terminal manager is shutting down.
	ErrManagerShuttingDown = errors.New("terminal manager is shutting down")
	// ErrReservationInactive indicates that a released or already-activated lease was reused.
	ErrReservationInactive     = errors.New("terminal session reservation is inactive")
	errTerminalShutdownTimeout = errors.New("terminal cleanup timed out after force kill")
)

// Manager tracks active terminal sessions and coordinates server shutdown.
//
// Slot availability and connection cleanup are tracked independently:
//   - activeSlots controls whether new sessions can be acquired. It is
//     decremented when the session event loop exits (via ReleaseSlot),
//     immediately freeing capacity for new connections.
//   - sessions/wg track connections that still need cleanup (process
//     termination, I/O drain). Shutdown waits for wg and can force-kill
//     connections still in the sessions map.
type Manager struct {
	ctx    context.Context
	cancel context.CancelFunc

	maxSessions int

	mu            sync.Mutex
	reservedSlots int
	activeSlots   int
	shuttingDown  bool
	sessions      map[string]*Connection
	wg            sync.WaitGroup
}

type leaseState uint8

const (
	leaseStateReserved     leaseState = iota + 1
	leaseStateActive                          // Slot occupied, connection in sessions map
	leaseStateSlotReleased                    // Slot freed, connection still cleaning up
	leaseStateReleased                        // Fully done
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
	if m.reservedSlots+m.activeSlots >= m.maxSessions {
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

	m.activeSlots++
	m.sessions[conn.ID] = conn
	m.wg.Add(1)
	l.connID = conn.ID
	l.state = leaseStateActive
	return nil
}

// ReleaseSlot frees the session slot so new connections can be accepted,
// without waiting for cleanup to finish. This should be called when the
// session event loop exits (before process termination and I/O drain).
// The connection remains in the sessions map for shutdown/force-kill
// until Release is called.
func (l *sessionLease) ReleaseSlot() {
	if l == nil || l.manager == nil {
		return
	}

	m := l.manager
	m.mu.Lock()
	defer m.mu.Unlock()

	switch l.state {
	case leaseStateReserved:
		if m.reservedSlots > 0 {
			m.reservedSlots--
		}
		l.state = leaseStateReleased
	case leaseStateActive:
		m.activeSlots--
		l.state = leaseStateSlotReleased
	case leaseStateSlotReleased, leaseStateReleased:
		// Already released or fully done — nothing to do.
	}
}

// Release signals that cleanup is fully complete. It removes the connection
// from the sessions map and signals the shutdown WaitGroup. If ReleaseSlot
// was not called, Release also frees the slot.
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
		// ReleaseSlot was never called; free the slot too.
		m.activeSlots--
		delete(m.sessions, l.connID)
		doneActive = true
		l.state = leaseStateReleased
	case leaseStateSlotReleased:
		// Slot already freed by ReleaseSlot; finish cleanup tracking.
		delete(m.sessions, l.connID)
		doneActive = true
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

	if waitForSignal(done, 0) {
		return nil
	}

	if delay, ok := forceKillDelay(ctx, processShutdownGrace); ok {
		if delay <= 0 {
			m.forceKillSessions()
			return waitForForcedCleanup(done, ctx)
		}

		timer := time.NewTimer(delay)
		defer timer.Stop()

		select {
		case <-done:
			return nil
		case <-timer.C:
			m.forceKillSessions()
			return waitForForcedCleanup(done, ctx)
		case <-ctx.Done():
			m.forceKillSessions()
			return waitForForcedCleanup(done, ctx)
		}
	}

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		m.forceKillSessions()
		return waitForForcedCleanup(done, ctx)
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

func (m *Manager) forceKillSessions() {
	for _, conn := range m.snapshotSessions() {
		conn.ForceKill()
	}
}

func forceKillDelay(ctx context.Context, reserve time.Duration) (time.Duration, bool) {
	if reserve <= 0 {
		return 0, true
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return 0, false
	}
	return time.Until(deadline) - reserve, true
}

func waitForForcedCleanup(done <-chan struct{}, ctx context.Context) error {
	if waitForSignal(done, 0) {
		return nil
	}
	if err := ctx.Err(); err != nil {
		if waitForSignal(done, 0) {
			return nil
		}
		return errors.Join(err, errTerminalShutdownTimeout)
	}

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		if waitForSignal(done, 0) {
			return nil
		}
		return errors.Join(ctx.Err(), errTerminalShutdownTimeout)
	}
}
