package errors_test

import (
	"fmt"
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
