// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build windows

package desktop

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

// sessionConnectState returns the WTS connect state of the session this
// process runs in: windows.WTSActive while a client or the console draws it,
// windows.WTSDisconnected after the Remote Desktop client has gone.
func sessionConnectState() (uint32, error) {
	var session uint32
	if err := windows.ProcessIdToSessionId(windows.GetCurrentProcessId(), &session); err != nil {
		return 0, err
	}
	var sessions *windows.WTS_SESSION_INFO
	var count uint32
	// Handle 0 is WTS_CURRENT_SERVER_HANDLE.
	if err := windows.WTSEnumerateSessions(0, 0, 1, &sessions, &count); err != nil {
		return 0, err
	}
	// WTSEnumerateSessions hands back a Win32 array of count entries, owned
	// by the system until WTSFreeMemory.
	defer windows.WTSFreeMemory(uintptr(unsafe.Pointer(sessions))) //nolint:gosec // the pointer came from WTSEnumerateSessions
	for _, info := range unsafe.Slice(sessions, count) {           //nolint:gosec // count is the array length WTSEnumerateSessions reported
		if info.SessionID == session {
			return info.State, nil
		}
	}
	return 0, errors.New("the current session is not listed")
}

// sessionProblemForState names a session that is not drawing. Only an active
// session has a display that renders and takes input; a connected,
// disconnected, or idle one keeps its last picture and its size, so the
// locked-screen and display-size checks say nothing is wrong.
func sessionProblemForState(state uint32) (Problem, bool) {
	if state == windows.WTSActive {
		return Problem{}, false
	}
	return Problem{
		Code:    problemSessionDisconnected,
		Message: "the remote desktop session is disconnected, so its display is not drawing; reconnect the session (or re-attach the child session) and keep it connected while computer steps run",
	}, true
}

// sessionProblem reports a session that is not drawing. When the system
// cannot say, the session is assumed to be drawing, like OnConsoleSession
// assumes the console: a lookup failure must not stop every step.
func sessionProblem() (Problem, bool) {
	state, err := sessionConnectState()
	if err != nil {
		return Problem{}, false
	}
	return sessionProblemForState(state)
}

// OnConsoleSession reports whether this process runs in the session attached
// to the physical console, where a person at the keyboard could collide with
// a step's input. A remote desktop session is not the console: its
// connection feeds the desktop synthetic pointer and keyboard updates of its
// own, so "nobody has touched the desktop" is never true there. When the
// system cannot say, the console is assumed, which keeps the guard.
func OnConsoleSession() bool {
	var session uint32
	if err := windows.ProcessIdToSessionId(windows.GetCurrentProcessId(), &session); err != nil {
		return true
	}
	console := windows.WTSGetActiveConsoleSessionId()
	// 0xFFFFFFFF means no session is attached to the console right now.
	if console == 0xFFFFFFFF {
		return true
	}
	return session == console
}
