package withstack_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cockroachdb/errors/errbase"
	"github.com/cockroachdb/errors/withstack"
)

type customMultiError struct {
	errs []error
}

func (c *customMultiError) Error() string {
	var msgs []string
	for _, e := range c.errs {
		msgs = append(msgs, e.Error())
	}
	return strings.Join(msgs, "; ")
}

func (c *customMultiError) Unwrap() []error {
	return c.errs
}

type customSingleError struct {
	msg string
}

func (c *customSingleError) Error() string {
	return c.msg
}

func TestWithStackMultiUnwrapSlice(t *testing.T) {
	err1 := errors.New("err one")
	err2 := errors.New("err two")
	multi := &customMultiError{errs: []error{err1, err2}}

	wrapped := withstack.WithStack(multi)
	if wrapped == nil {
		t.Fatalf("expected non-nil wrapped error")
	}

	// Verify type assertion to interface{ Unwrap() []error } succeeds without panic.
	u, ok := wrapped.(interface{ Unwrap() []error })
	if !ok {
		t.Fatalf("expected wrapped multi-error to implement interface{ Unwrap() []error }")
	}

	unwrapped := u.Unwrap()
	if len(unwrapped) != 2 {
		t.Fatalf("expected 2 unwrapped errors, got %d", len(unwrapped))
	}
	if unwrapped[0] != err1 || unwrapped[1] != err2 {
		t.Errorf("unexpected unwrapped errors: got %v, want [%v, %v]", unwrapped, err1, err2)
	}

	// Verify Cause() error is preserved.
	causer, ok := wrapped.(interface{ Cause() error })
	if !ok {
		t.Fatalf("expected wrapped multi-error to implement interface{ Cause() error }")
	}
	if causer.Cause() != multi {
		t.Errorf("expected cause %v, got %v", multi, causer.Cause())
	}

	// Verify StackTraceProvider.
	stProvider, ok := wrapped.(errbase.StackTraceProvider)
	if !ok {
		t.Fatalf("expected wrapped multi-error to implement StackTraceProvider")
	}
	st := stProvider.StackTrace()
	if len(st) == 0 {
		t.Errorf("expected non-empty stack trace")
	}

	// Verify SafeDetailer.
	detailer, ok := wrapped.(errbase.SafeDetailer)
	if !ok {
		t.Fatalf("expected wrapped multi-error to implement SafeDetailer")
	}
	details := detailer.SafeDetails()
	if len(details) == 0 || !strings.Contains(details[0], "TestWithStackMultiUnwrapSlice") {
		t.Errorf("expected safe details containing function name, got %v", details)
	}

	// Verify Error() formatting.
	if wrapped.Error() != multi.Error() {
		t.Errorf("expected Error() %q, got %q", multi.Error(), wrapped.Error())
	}

	// Verify Format with %+v includes attached stack trace.
	verbose := fmt.Sprintf("%+v", wrapped)
	if !strings.Contains(verbose, "TestWithStackMultiUnwrapSlice") {
		t.Errorf("expected verbose format to contain test name, got: %s", verbose)
	}
}

func TestWithStackSingleUnwrap(t *testing.T) {
	single := &customSingleError{msg: "single"}
	wrapped := withstack.WithStack(single)

	// Single error wrapper must implement Unwrap() error, NOT Unwrap() []error.
	if _, ok := wrapped.(interface{ Unwrap() []error }); ok {
		t.Fatalf("single error wrapper should not implement Unwrap() []error")
	}

	u, ok := wrapped.(interface{ Unwrap() error })
	if !ok {
		t.Fatalf("expected single error wrapper to implement Unwrap() error")
	}
	if u.Unwrap() != single {
		t.Errorf("expected unwrapped %v, got %v", single, u.Unwrap())
	}
}

func TestWithStackDepthNil(t *testing.T) {
	if err := withstack.WithStack(nil); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if err := withstack.WithStackDepth(nil, 2); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}
