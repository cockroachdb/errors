package errors_test

import (
	stderrors "errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/errors/testutils"
)

func TestUnwrap(t *testing.T) {
	tt := testutils.T{t}

	e := fmt.Errorf("foo %w %w", fmt.Errorf("bar"), fmt.Errorf("baz"))

	// Compatibility with go 1.20: Unwrap() on a multierror returns nil
	// (per API documentation)
	tt.Check(errors.Unwrap(e) == nil)
}

type sampleCustomError struct {
	id int
}

func (s *sampleCustomError) Error() string {
	return fmt.Sprintf("sample error %d", s.id)
}

// More detailed testing of Join is in datadriven_test.go. Here we make
// sure that the public API includes the stacktrace wrapper and supports
// the Go 1.20+ Unwrap() []error slice interface.
func TestJoin(t *testing.T) {
	err1 := errors.New("abc123")
	err2 := errors.New("def456")
	e := errors.Join(err1, err2)

	printed := fmt.Sprintf("%+v", e)
	expected := `Error types: (1) *withstack.withStackMulti (2) *join.joinError (3) *withstack.withStack (4) *errutil.leafError (5) *withstack.withStack (6) *errutil.leafError`
	if !strings.Contains(printed, expected) {
		t.Errorf("Expected: %s to contain: %s", printed, expected)
	}

	// Verify standard library errors.Join compatibility:
	// Calling err.(interface{ Unwrap() []error }).Unwrap() must not panic
	// and must return the slice of child errors.
	u, ok := e.(interface{ Unwrap() []error })
	if !ok {
		t.Fatalf("errors.Join result does not implement interface{ Unwrap() []error }")
	}
	causes := u.Unwrap()
	if len(causes) != 2 {
		t.Fatalf("expected 2 unwrapped causes, got %d", len(causes))
	}
	if causes[0] != err1 || causes[1] != err2 {
		t.Errorf("unexpected causes: %v, want [%v, %v]", causes, err1, err2)
	}

	// Verify errors.Is (cockroachdb) works through Join with stack trace.
	if !errors.Is(e, err1) {
		t.Errorf("expected errors.Is(e, err1) to be true")
	}
	if !errors.Is(e, err2) {
		t.Errorf("expected errors.Is(e, err2) to be true")
	}
	unrelatedErr := errors.New("unrelated")
	if errors.Is(e, unrelatedErr) {
		t.Errorf("expected errors.Is(e, unrelatedErr) to be false")
	}

	// Verify stdlib errors.Is works through Join with stack trace.
	if !stderrors.Is(e, err1) {
		t.Errorf("expected stderrors.Is(e, err1) to be true")
	}
	if !stderrors.Is(e, err2) {
		t.Errorf("expected stderrors.Is(e, err2) to be true")
	}

	// Verify errors.As and stdlib errors.As work through Join with stack trace.
	customErr := &sampleCustomError{id: 42}
	joinedWithCustom := errors.Join(err1, customErr)

	var asTarget *sampleCustomError
	if !errors.As(joinedWithCustom, &asTarget) {
		t.Fatalf("expected errors.As to find *sampleCustomError")
	}
	if asTarget == nil || asTarget.id != 42 {
		t.Errorf("expected asTarget.id == 42, got %v", asTarget)
	}

	var stdAsTarget *sampleCustomError
	if !stderrors.As(joinedWithCustom, &stdAsTarget) {
		t.Fatalf("expected stderrors.As to find *sampleCustomError")
	}
	if stdAsTarget == nil || stdAsTarget.id != 42 {
		t.Errorf("expected stdAsTarget.id == 42, got %v", stdAsTarget)
	}

	// Verify Join with nil values.
	if nilJoin := errors.Join(nil, nil); nilJoin != nil {
		t.Errorf("expected Join(nil, nil) == nil, got %v", nilJoin)
	}
	singleJoin := errors.Join(err1, nil)
	if !errors.Is(singleJoin, err1) {
		t.Errorf("expected singleJoin to match err1")
	}
	uSingle, ok := singleJoin.(interface{ Unwrap() []error })
	if !ok {
		t.Fatalf("singleJoin does not implement interface{ Unwrap() []error }")
	}
	if len(uSingle.Unwrap()) != 1 || uSingle.Unwrap()[0] != err1 {
		t.Errorf("unexpected uSingle.Unwrap(): %v", uSingle.Unwrap())
	}
}
