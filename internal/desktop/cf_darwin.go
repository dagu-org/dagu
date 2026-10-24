// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build darwin

package desktop

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

const libSystemPath = "/usr/lib/libSystem.B.dylib"

// Core Foundation number types and Accessibility value types.
const (
	cfNumberSInt64Type = 4
	cfNumberDoubleType = 13
	axValueCGPointType = 1
	axValueCGSizeType  = 2
	// procPIDPathMax is PROC_PIDPATHINFO_MAXSIZE.
	procPIDPathMax = 4096
)

// Accessibility error codes.
const (
	axErrorSuccess               = 0
	axErrorAttributeUnsupported  = -25205
	axErrorNoValue               = -25212
	axErrorActionUnsupported     = -25206
	axErrorCannotComplete        = -25204
	axErrorNotImplemented        = -25208
	axErrorIllegalArgument       = -25201
	axErrorInvalidUIElement      = -25202
	axErrorAPIDisabled           = -25211
	axErrorNotificationsDisabled = -25213
)

// Functions that read windows and their attributes, bound when the
// libraries load.
var (
	cfGetTypeID                            func(ref uintptr) uintptr
	cfRetain                               func(ref uintptr) uintptr
	cfStringGetTypeID                      func() uintptr
	cfNumberGetTypeID                      func() uintptr
	cfBooleanGetTypeID                     func() uintptr
	cfArrayGetTypeID                       func() uintptr
	cfStringGetLength                      func(text uintptr) int
	cfStringGetMaximumSizeForEncoding      func(length int, encoding uint32) int
	cfStringGetCString                     func(text uintptr, buffer *byte, size int, encoding uint32) bool
	cfArrayGetCount                        func(array uintptr) int
	cfArrayGetValueAtIndex                 func(array uintptr, index int) uintptr
	cfNumberGetInt64                       func(number uintptr, kind int, out *int64) bool
	cfNumberGetFloat64                     func(number uintptr, kind int, out *float64) bool
	cgRectMakeWithDictionaryRepresentation func(dict uintptr, rect *cgRect) bool
	cgWindowListCopyWindowInfo             func(option uint32, relativeToWindow uint32) uintptr
	axUIElementCreateSystemWide            func() uintptr
	axUIElementCreateApplication           func(pid int32) uintptr
	axUIElementCopyAttributeValue          func(element, attribute uintptr, value *uintptr) int32
	axUIElementSetAttributeValue           func(element, attribute, value uintptr) int32
	axUIElementIsAttributeSettable         func(element, attribute uintptr, settable *uint8) int32
	axUIElementPerformAction               func(element, action uintptr) int32
	axUIElementGetPid                      func(element uintptr, pid *int32) int32
	axUIElementSetMessagingTimeout         func(element uintptr, seconds float32) int32
	axValueCreatePoint                     func(kind uint32, value *cgPoint) uintptr
	axValueCreateSize                      func(kind uint32, value *cgSize) uintptr
	axValueGetPoint                        func(value uintptr, kind uint32, out *cgPoint) bool
	axValueGetSize                         func(value uintptr, kind uint32, out *cgSize) bool
	procPIDPath                            func(pid int32, buffer *byte, size uint32) int32
)

// Keys of the window server's window descriptions, resolved when the
// libraries load.
var (
	cgWindowNumberKey   uintptr
	cgWindowOwnerPIDKey uintptr
	cgWindowNameKey     uintptr
	cgWindowLayerKey    uintptr
	cgWindowBoundsKey   uintptr
	cgWindowAlphaKey    uintptr
)

// loadWindowFuncs binds what reading windows and their attributes needs.
func loadWindowFuncs(cg, cf, as uintptr) error {
	purego.RegisterLibFunc(&cfGetTypeID, cf, "CFGetTypeID")
	purego.RegisterLibFunc(&cfRetain, cf, "CFRetain")
	purego.RegisterLibFunc(&cfStringGetTypeID, cf, "CFStringGetTypeID")
	purego.RegisterLibFunc(&cfNumberGetTypeID, cf, "CFNumberGetTypeID")
	purego.RegisterLibFunc(&cfBooleanGetTypeID, cf, "CFBooleanGetTypeID")
	purego.RegisterLibFunc(&cfArrayGetTypeID, cf, "CFArrayGetTypeID")
	purego.RegisterLibFunc(&cfStringGetLength, cf, "CFStringGetLength")
	purego.RegisterLibFunc(&cfStringGetMaximumSizeForEncoding, cf, "CFStringGetMaximumSizeForEncoding")
	purego.RegisterLibFunc(&cfStringGetCString, cf, "CFStringGetCString")
	purego.RegisterLibFunc(&cfArrayGetCount, cf, "CFArrayGetCount")
	purego.RegisterLibFunc(&cfArrayGetValueAtIndex, cf, "CFArrayGetValueAtIndex")
	purego.RegisterLibFunc(&cfNumberGetInt64, cf, "CFNumberGetValue")
	purego.RegisterLibFunc(&cfNumberGetFloat64, cf, "CFNumberGetValue")
	purego.RegisterLibFunc(&cgRectMakeWithDictionaryRepresentation, cg, "CGRectMakeWithDictionaryRepresentation")
	purego.RegisterLibFunc(&cgWindowListCopyWindowInfo, cg, "CGWindowListCopyWindowInfo")
	purego.RegisterLibFunc(&axUIElementCreateSystemWide, as, "AXUIElementCreateSystemWide")
	purego.RegisterLibFunc(&axUIElementCreateApplication, as, "AXUIElementCreateApplication")
	purego.RegisterLibFunc(&axUIElementCopyAttributeValue, as, "AXUIElementCopyAttributeValue")
	purego.RegisterLibFunc(&axUIElementSetAttributeValue, as, "AXUIElementSetAttributeValue")
	purego.RegisterLibFunc(&axUIElementIsAttributeSettable, as, "AXUIElementIsAttributeSettable")
	purego.RegisterLibFunc(&axUIElementPerformAction, as, "AXUIElementPerformAction")
	purego.RegisterLibFunc(&axUIElementGetPid, as, "AXUIElementGetPid")
	purego.RegisterLibFunc(&axUIElementSetMessagingTimeout, as, "AXUIElementSetMessagingTimeout")
	purego.RegisterLibFunc(&axValueCreatePoint, as, "AXValueCreate")
	purego.RegisterLibFunc(&axValueCreateSize, as, "AXValueCreate")
	purego.RegisterLibFunc(&axValueGetPoint, as, "AXValueGetValue")
	purego.RegisterLibFunc(&axValueGetSize, as, "AXValueGetValue")
	libSystem, err := purego.Dlopen(libSystemPath, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return fmt.Errorf("load libSystem: %w", err)
	}
	purego.RegisterLibFunc(&procPIDPath, libSystem, "proc_pidpath")
	for _, key := range []struct {
		name string
		ref  *uintptr
	}{
		{"kCGWindowNumber", &cgWindowNumberKey},
		{"kCGWindowOwnerPID", &cgWindowOwnerPIDKey},
		{"kCGWindowName", &cgWindowNameKey},
		{"kCGWindowLayer", &cgWindowLayerKey},
		{"kCGWindowBounds", &cgWindowBoundsKey},
		{"kCGWindowAlpha", &cgWindowAlphaKey},
	} {
		symbol, err := purego.Dlsym(cg, key.name)
		if err != nil {
			return fmt.Errorf("load %s: %w", key.name, err)
		}
		// The symbol is the address of the variable that holds the string.
		*key.ref = *(*uintptr)(unsafe.Add(nil, symbol)) //nolint:gosec // reads a C global whose address dlsym returned
	}
	return nil
}

// cfNames holds the Core Foundation strings of attribute and action names,
// made once and kept for the life of the process.
var (
	cfNamesMu sync.Mutex
	cfNames   = map[string]uintptr{}
)

// cfName returns a Core Foundation string for a constant name, such as an
// attribute's. The string is shared and never released.
func cfName(name string) uintptr {
	cfNamesMu.Lock()
	defer cfNamesMu.Unlock()
	if ref, ok := cfNames[name]; ok {
		return ref
	}
	ref := cfString(name)
	if ref != 0 {
		cfNames[name] = ref
	}
	return ref
}

// goString reads a Core Foundation string, or returns "" for anything else.
func goString(ref uintptr) string {
	if ref == 0 || cfGetTypeID(ref) != cfStringGetTypeID() {
		return ""
	}
	size := cfStringGetMaximumSizeForEncoding(cfStringGetLength(ref), cfStringEncodingUTF8) + 1
	buffer := make([]byte, size)
	if !cfStringGetCString(ref, &buffer[0], size, cfStringEncodingUTF8) {
		return ""
	}
	for i, b := range buffer {
		if b == 0 {
			return string(buffer[:i])
		}
	}
	return string(buffer)
}

// cfInt reads a Core Foundation number as an integer.
func cfInt(ref uintptr) (int64, bool) {
	if ref == 0 || cfGetTypeID(ref) != cfNumberGetTypeID() {
		return 0, false
	}
	var value int64
	ok := cfNumberGetInt64(ref, cfNumberSInt64Type, &value)
	return value, ok
}

// cfFloat reads a Core Foundation number as a float.
func cfFloat(ref uintptr) (float64, bool) {
	if ref == 0 || cfGetTypeID(ref) != cfNumberGetTypeID() {
		return 0, false
	}
	var value float64
	ok := cfNumberGetFloat64(ref, cfNumberDoubleType, &value)
	return value, ok
}

// cfBool reads a Core Foundation boolean, or a number used as one.
func cfBool(ref uintptr) (bool, bool) {
	if ref == 0 {
		return false, false
	}
	switch cfGetTypeID(ref) {
	case cfBooleanGetTypeID():
		return cfBooleanGetValue(ref), true
	case cfNumberGetTypeID():
		n, ok := cfInt(ref)
		return n != 0, ok
	}
	return false, false
}

// cfArray lists the values of a Core Foundation array, which the array
// keeps alive.
func cfArray(ref uintptr) []uintptr {
	if ref == 0 || cfGetTypeID(ref) != cfArrayGetTypeID() {
		return nil
	}
	count := cfArrayGetCount(ref)
	values := make([]uintptr, 0, count)
	for i := range count {
		values = append(values, cfArrayGetValueAtIndex(ref, i))
	}
	return values
}

// axCopy reads an attribute of an Accessibility element. The caller
// releases a non-zero value.
func axCopy(element uintptr, attribute string) uintptr {
	if element == 0 {
		return 0
	}
	var value uintptr
	if axUIElementCopyAttributeValue(element, cfName(attribute), &value) != axErrorSuccess {
		return 0
	}
	return value
}

// axString reads a string attribute.
func axString(element uintptr, attribute string) string {
	value := axCopy(element, attribute)
	if value == 0 {
		return ""
	}
	defer cfRelease(value)
	return goString(value)
}

// axBool reads a boolean attribute. ok is false when the element does not
// have it.
func axBool(element uintptr, attribute string) (value, ok bool) {
	ref := axCopy(element, attribute)
	if ref == 0 {
		return false, false
	}
	defer cfRelease(ref)
	return cfBool(ref)
}

// axPoint reads a point attribute, such as a position, in points.
func axPoint(element uintptr, attribute string) (cgPoint, bool) {
	ref := axCopy(element, attribute)
	if ref == 0 {
		return cgPoint{}, false
	}
	defer cfRelease(ref)
	var point cgPoint
	ok := axValueGetPoint(ref, axValueCGPointType, &point)
	return point, ok
}

// axSize reads a size attribute in points.
func axSize(element uintptr, attribute string) (cgSize, bool) {
	ref := axCopy(element, attribute)
	if ref == 0 {
		return cgSize{}, false
	}
	defer cfRelease(ref)
	var size cgSize
	ok := axValueGetSize(ref, axValueCGSizeType, &size)
	return size, ok
}

// axFrame reads an element's frame in points.
func axFrame(element uintptr) (cgRect, bool) {
	position, ok := axPoint(element, "AXPosition")
	if !ok {
		return cgRect{}, false
	}
	size, ok := axSize(element, "AXSize")
	if !ok {
		return cgRect{}, false
	}
	return cgRect{Origin: position, Size: size}, true
}

// axSettable reports whether an attribute of an element can be set.
func axSettable(element uintptr, attribute string) bool {
	var settable uint8
	return axUIElementIsAttributeSettable(element, cfName(attribute), &settable) == axErrorSuccess && settable != 0
}

// axSet sets an attribute to a Core Foundation value.
func axSet(element uintptr, attribute string, value uintptr) error {
	if value == 0 {
		return fmt.Errorf("set %s: no value", attribute)
	}
	if code := axUIElementSetAttributeValue(element, cfName(attribute), value); code != axErrorSuccess {
		return axError("set "+attribute, code)
	}
	return nil
}

// axSetPoint sets a point attribute, such as a position, in points.
func axSetPoint(element uintptr, attribute string, point cgPoint) error {
	value := axValueCreatePoint(axValueCGPointType, &point)
	if value == 0 {
		return errors.New("create a point value")
	}
	defer cfRelease(value)
	return axSet(element, attribute, value)
}

// axSetSize sets a size attribute in points.
func axSetSize(element uintptr, attribute string, size cgSize) error {
	value := axValueCreateSize(axValueCGSizeType, &size)
	if value == 0 {
		return errors.New("create a size value")
	}
	defer cfRelease(value)
	return axSet(element, attribute, value)
}

// axPerform performs an action of an element.
func axPerform(element uintptr, action string) error {
	if code := axUIElementPerformAction(element, cfName(action)); code != axErrorSuccess {
		return axError(action, code)
	}
	return nil
}

// axError describes an Accessibility error code.
func axError(what string, code int32) error {
	var reason string
	switch code {
	case axErrorAttributeUnsupported:
		reason = "the attribute is not supported"
	case axErrorActionUnsupported:
		reason = "the action is not supported"
	case axErrorNoValue:
		reason = "there is no value"
	case axErrorCannotComplete:
		reason = "the application did not answer"
	case axErrorNotImplemented:
		reason = "the application does not implement it"
	case axErrorIllegalArgument:
		reason = "the value is not accepted"
	case axErrorInvalidUIElement:
		reason = "the element is gone"
	case axErrorAPIDisabled:
		reason = "Accessibility permission is missing"
	case axErrorNotificationsDisabled:
		reason = "notifications are disabled"
	default:
		reason = fmt.Sprintf("AXError %d", code)
	}
	return fmt.Errorf("%s: %s", what, reason)
}
