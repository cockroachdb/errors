package errors_test

import (
	"fmt"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/errors/testutils"
	"github.com/cockroachdb/redact"
)

func TestUnwrap(t *testing.T) {
	tt := testutils.T{t}

	e := fmt.Errorf("foo %w %w", fmt.Errorf("bar"), fmt.Errorf("baz"))

	// Compatibility with go 1.20: Unwrap() on a multierror returns nil
	// (per API documentation)
	tt.Check(errors.Unwrap(e) == nil)
}

func TestJoin(t *testing.T) {
	e := errors.Join(errors.New("abc123"), errors.New("def456"))
	printed := redact.Sprintf("%+v", e)
	expectedR := redact.RedactableString(`‹abc123›
(1) attached stack trace
  -- stack trace:
  | github.com/cockroachdb/errors_test.TestJoin
  | 	/Users/davidh/go/src/github.com/cockroachdb/errors/errutil_api_test.go:23
  | testing.tRunner
  | 	/opt/homebrew/Cellar/go/1.20.3/libexec/src/testing/testing.go:1576
Wraps: (2) ‹abc123›
  | ‹def456›
└─ Wraps: (3) attached stack trace
  -- stack trace:
  | github.com/cockroachdb/errors_test.TestJoin
  | 	/Users/davidh/go/src/github.com/cockroachdb/errors/errutil_api_test.go:23
  | [...repeated from below...]
  └─ Wraps: (4) def456
└─ Wraps: (5) attached stack trace
  -- stack trace:
  | github.com/cockroachdb/errors_test.TestJoin
  | 	/Users/davidh/go/src/github.com/cockroachdb/errors/errutil_api_test.go:23
  | testing.tRunner
  | 	/opt/homebrew/Cellar/go/1.20.3/libexec/src/testing/testing.go:1576
  | runtime.goexit
  | 	/opt/homebrew/Cellar/go/1.20.3/libexec/src/runtime/asm_arm64.s:1172
  └─ Wraps: (6) abc123
Error types: (1) *withstack.withStack (2) *join.joinError (3) *withstack.withStack (4) *errutil.leafError (5) *withstack.withStack (6) *errutil.leafError`)
	if printed != expectedR {
		t.Errorf("Expected: %s; Got: %s", expectedR, printed)
	}
}
