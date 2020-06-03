package sentry

import (
	"go/build"
	"strings"
)

func isInAppFrame(frame Frame) bool {
	if frame.Module == "main" {
		return true
	}
	// If the client is overriding the in-app detection logic, use that.
	if isInAppFrameFn != nil {
		return isInAppFrameFn(frame)
	}

	// Legacy sentry-go behavior.
	if strings.HasPrefix(frame.AbsPath, build.Default.GOROOT) ||
		strings.Contains(frame.Module, "/vendor/") ||
		strings.Contains(frame.Module, "/third_party/") {
		return false
	}

	return true
}

var isInAppFrameFn func(Frame) bool

// RegisterInAppFrameFn can be used by clients to customize the logic
// that decides whether a frame is considered "in-app" for event
// reporting.
func RegisterInAppFrameFn(fn func(Frame) bool) {
	isInAppFrameFn = fn
}
