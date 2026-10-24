// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package computer

import (
	"fmt"
	"image"
	"os"
	"strings"

	"github.com/dagucloud/dagu/v2/internal/desktop"
	"github.com/dagucloud/dagu/v2/internal/llm/computeruse"
)

// protectedAppsEnv lists applications, by executable name and separated by
// commas, whose windows a step never operates or maximizes, such as the one
// a person starts the automation from.
const protectedAppsEnv = "DAGU_DESKTOP_PROTECTED_APPS"

// windowSizeTolerance is how many pixels a window's size may differ from the
// recorded one and still be the window a turn was recorded on.
const windowSizeTolerance = 2

// protectedApps reads the applications a step never operates.
func protectedApps() []string {
	return strings.Split(os.Getenv(protectedAppsEnv), ",")
}

// recordedWindow is the application window a pixel turn was recorded on.
// The turn fingerprints only this window, and its actions are relative to
// the window's top left, so a replay finds the window wherever it is now and
// whatever surrounds it. A replay knows the window by its application and
// size; the title only prefers one of several such windows.
type recordedWindow struct {
	App    string `json:"app"`
	Title  string `json:"title,omitempty"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// view is the part of the display a pixel turn is compared on: the window it
// was recorded on, or the whole display where the system names no window.
type view struct {
	rect   image.Rectangle
	window *recordedWindow
}

// displayView compares the whole display, with actions in display pixels.
func displayView(full *image.RGBA) view {
	return view{rect: full.Bounds()}
}

// region is the part of an image the view compares. It shares the image's
// pixels and coordinates; the sub-image of an RGBA is always an RGBA.
func (v view) region(img *image.RGBA) *image.RGBA {
	sub, _ := img.SubImage(v.rect.Intersect(img.Bounds())).(*image.RGBA)
	return sub
}

// fingerprint hashes what the view shows of an image.
func (v view) fingerprint(img *image.RGBA) desktop.Fingerprint {
	return desktop.FingerprintOf(v.region(img))
}

// targetPrint hashes the area around a display position, inside the view.
func (v view) targetPrint(img *image.RGBA, at image.Point) desktop.Fingerprint {
	return desktop.FingerprintAround(v.region(img), at, targetRadius(v.rect))
}

// relative moves an action from display pixels to the view's own.
func (v view) relative(action computeruse.Action) computeruse.Action {
	if v.window == nil {
		return action
	}
	return shifted(action, image.Point{}.Sub(v.rect.Min))
}

// absolute moves an action from the view's pixels to the display's.
func (v view) absolute(action computeruse.Action) computeruse.Action {
	if v.window == nil {
		return action
	}
	return shifted(action, v.rect.Min)
}

// place records an action the model ran in display pixels within the view:
// relative to the view, with the area it lands on fingerprinted on the
// screen the model chose it on.
func (v view) place(rec recordedAction, seen *image.RGBA) recordedAction {
	if at, ok := target(rec.Action); ok {
		fingerprint := v.targetPrint(seen, at)
		rec.Target = &fingerprint
	}
	rec.Action = v.relative(rec.Action)
	return rec
}

// shifted moves an action's positions by offset.
func shifted(action computeruse.Action, offset image.Point) computeruse.Action {
	if action.Point != nil {
		action.Point = &computeruse.Point{X: action.Point.X + offset.X, Y: action.Point.Y + offset.Y}
	}
	if len(action.Path) > 0 {
		path := make([]computeruse.Point, len(action.Path))
		for i, p := range action.Path {
			path[i] = computeruse.Point{X: p.X + offset.X, Y: p.Y + offset.Y}
		}
		action.Path = path
	}
	return action
}

// windowView is the view of the application window w is part of: w itself
// when it is a top-level window, or the front window of its application
// when w is a menu or another window the system does not list. It is the
// whole display when the system does not name the window.
func (r *run) windowView(w desktop.WindowID, display view) view {
	w, ok := r.appWindow(w)
	if !ok || w.Bounds.Intersect(display.rect).Empty() {
		return display
	}
	return view{rect: w.Bounds, window: &recordedWindow{App: w.App, Title: w.Title, Width: w.Bounds.Dx(), Height: w.Bounds.Dy()}}
}

// appWindow returns the listed top-level window w belongs to.
func (r *run) appWindow(w desktop.WindowID) (desktop.WindowID, bool) {
	if !w.Known() || w.App == "" || w.Bounds.Empty() {
		return desktop.WindowID{}, false
	}
	windows := r.driver.Windows()
	for _, c := range windows {
		if c.Handle == w.Handle {
			return c, !c.Bounds.Empty()
		}
	}
	for _, c := range windows {
		if c.PID == w.PID && c.App != "" && !c.Bounds.Empty() {
			return c, true
		}
	}
	return desktop.WindowID{}, false
}

// findWindow finds the window a turn was recorded on: the front window of
// its application with its size, preferring one with its title.
func (r *run) findWindow(rec recordedWindow) (image.Rectangle, bool) {
	var bySize image.Rectangle
	for _, w := range r.driver.Windows() {
		if !strings.EqualFold(w.App, rec.App) || r.driver.Owned(w) ||
			abs(w.Bounds.Dx()-rec.Width) > windowSizeTolerance || abs(w.Bounds.Dy()-rec.Height) > windowSizeTolerance {
			continue
		}
		if w.Title == rec.Title {
			return w.Bounds, true
		}
		if bySize.Empty() {
			bySize = w.Bounds
		}
	}
	return bySize, !bySize.Empty()
}

// fit maximizes the front window when the operation running asks for it.
// Each window is maximized at most once in a step attempt, so one a person
// or its application resizes afterwards is left as it is. A window the step
// never operates, one that cannot be maximized, and one already maximized
// are left alone.
func (r *run) fit(index int) {
	if !r.maximize || r.driver == nil {
		return
	}
	w := r.driver.FocusedWindow()
	if !w.Known() || r.driver.Owned(w) || r.fitted[w.Handle] {
		return
	}
	r.fitted[w.Handle] = true
	done, err := r.driver.Maximize(w)
	switch {
	case err != nil:
		logAction(r.timeline, index, fmt.Sprintf("could not maximize %s: %s", describeWindow(w), r.masker.MaskString(err.Error())))
	case done:
		logAction(r.timeline, index, "maximized "+describeWindow(w))
	}
}

// describeWindow names a window for the step log.
func describeWindow(w desktop.WindowID) string {
	if w.Title == "" {
		return w.App
	}
	return fmt.Sprintf("%s %q", w.App, w.Title)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
