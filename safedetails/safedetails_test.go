// Copyright 2019 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package safedetails_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/cockroachdb/errors/errbase"
	"github.com/cockroachdb/errors/markers"
	"github.com/cockroachdb/errors/safedetails"
	"github.com/cockroachdb/errors/testutils"
)

func TestDetailCapture(t *testing.T) {
	origErr := errors.New("hello world")

	err := safedetails.WithSafeDetails(origErr, "bye %s %s", safedetails.Safe("planet"), "and universe")

	t.Logf("here's the error:\n%+v", err)

	subTest := func(t *testing.T, err error) {
		tt := testutils.T{T: t}

		// The cause is preserved.
		tt.Check(markers.Is(err, origErr))

		// The message is unchanged by the wrapper.
		tt.CheckEqual(err.Error(), "hello world")

		// The unsafe string is hidden.
		errV := fmt.Sprintf("%+v", err)
		tt.Check(!strings.Contains(errV, "and universe"))

		// The safe string is preserved.
		tt.Check(strings.Contains(errV, "planet"))

		// The format string is preserved.
		tt.Check(strings.Contains(errV, "bye %s %s"))
	}

	// Check the error properties locally.
	t.Run("local", func(t *testing.T) {
		subTest(t, err)
	})

	// Same tests, across the network.
	enc := errbase.EncodeError(context.Background(), err)
	newErr := errbase.DecodeError(context.Background(), enc)

	t.Run("remote", func(t *testing.T) {
		subTest(t, newErr)
	})
}

func TestFormat(t *testing.T) {
	tt := testutils.T{t}

	baseErr := errors.New("woo")
	const woo = `woo`
	const waawoo = `waa: woo`
	testCases := []struct {
		name          string
		err           error
		expFmtSimple  string
		expFmtVerbose string
	}{
		{"safe onearg",
			safedetails.WithSafeDetails(baseErr, "a"),
			woo, `
error with embedded safe details: a
  - woo`},

		{"safe empty",
			safedetails.WithSafeDetails(baseErr, ""),
			woo, `
safe detail wrapper with no details
  - woo`},

		{"safe nofmt+onearg",
			safedetails.WithSafeDetails(baseErr, "", 123),
			woo, `
error with embedded safe details:
    -- arg 1: <int>
  - woo`},

		{"safe err",
			safedetails.WithSafeDetails(baseErr, "a %v",
				&os.PathError{
					Op:   "open",
					Path: "/hidden",
					Err:  os.ErrNotExist,
				}),
			woo, `
error with embedded safe details: a %v
    -- arg 1: *errors.errorString: file does not exist
    wrapper: *os.PathError: open
  - woo`},

		{"safe",
			safedetails.WithSafeDetails(baseErr, "a %s %s", "b", safedetails.Safe("c")),
			woo, `
error with embedded safe details: a %s %s
    -- arg 1: <string>
    -- arg 2: c
  - woo`},

		{"safe + wrapper",
			safedetails.WithSafeDetails(&werrFmt{baseErr, "waa"}, "a %s %s", "b", safedetails.Safe("c")),
			waawoo, `
error with embedded safe details: a %s %s
    -- arg 1: <string>
    -- arg 2: c
  - waa:
    -- verbose wrapper:
    waa
  - woo`},

		{"wrapper + safe",
			&werrFmt{safedetails.WithSafeDetails(baseErr, "a %s %s", "b", safedetails.Safe("c")), "waa"},
			waawoo, `
waa:
    -- verbose wrapper:
    waa
  - error with embedded safe details: a %s %s
    -- arg 1: <string>
    -- arg 2: c
  - woo`},

		{"safe with wrapped error",
			safedetails.WithSafeDetails(baseErr, "a %v",
				&werrFmt{errors.New("wuu"), "waa"}),
			woo,
			`error with embedded safe details: a %v
    -- arg 1: <*errors.errorString>
    wrapper: <*safedetails_test.werrFmt>
  - woo`},

		{"safe in safe",
			safedetails.WithSafeDetails(baseErr, "a %v",
				safedetails.WithSafeDetails(errors.New("wuu"),
					"b %v", safedetails.Safe("waa"))),
			woo,
			`error with embedded safe details: a %v
    -- arg 1: <*errors.errorString>
    wrapper: <*safedetails.withSafeDetails>
    (more details:)
    b %v
    -- arg 1: waa
  - woo`},
	}

	for _, test := range testCases {
		tt.Run(test.name, func(tt testutils.T) {
			err := test.err

			// %s is simple formatting
			tt.CheckEqual(fmt.Sprintf("%s", err), test.expFmtSimple)

			// %v is simple formatting too, for compatibility with the past.
			tt.CheckEqual(fmt.Sprintf("%v", err), test.expFmtSimple)

			// %q is always like %s but quotes the result.
			ref := fmt.Sprintf("%q", test.expFmtSimple)
			tt.CheckEqual(fmt.Sprintf("%q", err), ref)

			// %+v is the verbose mode.
			refV := strings.TrimPrefix(test.expFmtVerbose, "\n")
			spv := fmt.Sprintf("%+v", err)
			tt.CheckEqual(spv, refV)
		})
	}
}

type werrFmt struct {
	cause error
	msg   string
}

var _ errbase.Formatter = (*werrFmt)(nil)

func (e *werrFmt) Error() string                 { return fmt.Sprintf("%s: %v", e.msg, e.cause) }
func (e *werrFmt) Unwrap() error                 { return e.cause }
func (e *werrFmt) Format(s fmt.State, verb rune) { errbase.FormatError(e, s, verb) }
func (e *werrFmt) FormatError(p errbase.Printer) error {
	p.Print(e.msg)
	if p.Detail() {
		p.Printf("-- verbose wrapper:\n%s", e.msg)
	}
	return e.cause
}
