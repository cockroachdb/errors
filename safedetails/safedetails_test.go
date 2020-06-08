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

	"github.com/interspace/errors/errbase"
	"github.com/interspace/errors/markers"
	"github.com/interspace/errors/safedetails"
	"github.com/interspace/errors/testutils"
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
		tt.CheckStringEqual(err.Error(), "hello world")

		// The detail strings are hidden.
		errV := fmt.Sprintf("%+v", err)
		tt.Check(!strings.Contains(errV, "and universe"))
		tt.Check(!strings.Contains(errV, "planet"))
		tt.Check(!strings.Contains(errV, "bye %s %s"))

		// The fact there are details is preserved.
		tt.Check(strings.Contains(errV, "3 safe details enclosed"))
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
		details       string
	}{
		{"safe onearg",
			safedetails.WithSafeDetails(baseErr, "a"),
			woo, `
woo
(1) 1 safe detail enclosed
Wraps: (2) woo
Error types: (1) *safedetails.withSafeDetails (2) *errors.errorString`,
			// Payload
			`payload 0
(0) a
payload 1
(empty)
`},

		{"safe empty",
			safedetails.WithSafeDetails(baseErr, ""),
			woo, `
woo
(1) 1 safe detail enclosed
Wraps: (2) woo
Error types: (1) *safedetails.withSafeDetails (2) *errors.errorString`,
			// Payload
			`payload 0
(empty)
payload 1
(empty)
`},

		{"safe nofmt+onearg",
			safedetails.WithSafeDetails(baseErr, "", 123),
			woo, `
woo
(1) 2 safe details enclosed
Wraps: (2) woo
Error types: (1) *safedetails.withSafeDetails (2) *errors.errorString`,
			// Payload
			`payload 0
(1) -- arg 1: <int>
payload 1
(empty)
`},

		{"safe err",
			safedetails.WithSafeDetails(baseErr, "a %v",
				&os.PathError{
					Op:   "open",
					Path: "/hidden",
					Err:  os.ErrNotExist,
				}),
			woo, `
woo
(1) 2 safe details enclosed
Wraps: (2) woo
Error types: (1) *safedetails.withSafeDetails (2) *errors.errorString`,
			// Payload
			`payload 0
(0) a %v
(1) -- arg 1: *errors.errorString: file does not exist
wrapper: *os.PathError: open
payload 1
(empty)
`},

		{"safe",
			safedetails.WithSafeDetails(baseErr, "a %s %s", "b", safedetails.Safe("c")),
			woo, `
woo
(1) 3 safe details enclosed
Wraps: (2) woo
Error types: (1) *safedetails.withSafeDetails (2) *errors.errorString`,
			// Payload
			`payload 0
(0) a %s %s
(1) -- arg 1: <string>
(2) -- arg 2: c
payload 1
(empty)
`},

		{"safe + wrapper",
			safedetails.WithSafeDetails(&werrFmt{baseErr, "waa"}, "a %s %s", "b", safedetails.Safe("c")),
			waawoo, `
waa: woo
(1) 3 safe details enclosed
Wraps: (2) waa
  | -- this is waa's
  | multi-line payload
Wraps: (3) woo
Error types: (1) *safedetails.withSafeDetails (2) *safedetails_test.werrFmt (3) *errors.errorString`,
			// Payload
			`payload 0
(0) a %s %s
(1) -- arg 1: <string>
(2) -- arg 2: c
payload 1
(empty)
payload 2
(empty)
`},

		{"wrapper + safe",
			&werrFmt{safedetails.WithSafeDetails(baseErr, "a %s %s", "b", safedetails.Safe("c")), "waa"},
			waawoo, `
waa: woo
(1) waa
  | -- this is waa's
  | multi-line payload
Wraps: (2) 3 safe details enclosed
Wraps: (3) woo
Error types: (1) *safedetails_test.werrFmt (2) *safedetails.withSafeDetails (3) *errors.errorString`,
			// Payload
			`payload 0
(empty)
payload 1
(0) a %s %s
(1) -- arg 1: <string>
(2) -- arg 2: c
payload 2
(empty)
`},

		{"safe with wrapped error",
			safedetails.WithSafeDetails(baseErr, "a %v",
				&werrFmt{errors.New("wuu"), "waa"}),
			woo,
			`
woo
(1) 2 safe details enclosed
Wraps: (2) woo
Error types: (1) *safedetails.withSafeDetails (2) *errors.errorString`,
			// Payload
			`payload 0
(0) a %v
(1) -- arg 1: <*errors.errorString>
wrapper: <*safedetails_test.werrFmt>
payload 1
(empty)
`},

		{"safe + safe, stacked",
			safedetails.WithSafeDetails(
				safedetails.WithSafeDetails(baseErr, "hello %s", safedetails.Safe("world")),
				"delicious %s", safedetails.Safe("coffee")),
			woo,
			`
woo
(1) 2 safe details enclosed
Wraps: (2) 2 safe details enclosed
Wraps: (3) woo
Error types: (1) *safedetails.withSafeDetails (2) *safedetails.withSafeDetails (3) *errors.errorString`,
			// Payload
			`payload 0
(0) delicious %s
(1) -- arg 1: coffee
payload 1
(0) hello %s
(1) -- arg 1: world
payload 2
(empty)
`},

		{"safe as arg to safe",
			safedetails.WithSafeDetails(baseErr, "a %v",
				safedetails.WithSafeDetails(errors.New("wuu"),
					"b %v", safedetails.Safe("waa"))),
			woo,
			`
woo
(1) 2 safe details enclosed
Wraps: (2) woo
Error types: (1) *safedetails.withSafeDetails (2) *errors.errorString`,
			// Payload
			`payload 0
(0) a %v
(1) -- arg 1: <*errors.errorString>
wrapper: <*safedetails.withSafeDetails>
(more details:)
b %v
-- arg 1: waa
payload 1
(empty)
`},
	}

	for _, test := range testCases {
		tt.Run(test.name, func(tt testutils.T) {
			err := test.err

			// %s is simple formatting
			tt.CheckStringEqual(fmt.Sprintf("%s", err), test.expFmtSimple)

			// %v is simple formatting too, for compatibility with the past.
			tt.CheckStringEqual(fmt.Sprintf("%v", err), test.expFmtSimple)

			// %q is always like %s but quotes the result.
			ref := fmt.Sprintf("%q", test.expFmtSimple)
			tt.CheckStringEqual(fmt.Sprintf("%q", err), ref)

			// %+v is the verbose mode.
			refV := strings.TrimPrefix(test.expFmtVerbose, "\n")
			spv := fmt.Sprintf("%+v", err)
			tt.CheckStringEqual(spv, refV)

			// Check the actual details produced.
			details := errbase.GetAllSafeDetails(err)
			var buf strings.Builder
			for i, d := range details {
				fmt.Fprintf(&buf, "payload %d\n", i)
				if len(d.SafeDetails) == 0 || (len(d.SafeDetails) == 1 && d.SafeDetails[0] == "") {
					fmt.Fprintf(&buf, "(empty)\n")
					continue
				}
				for j, sd := range d.SafeDetails {
					if len(sd) == 0 {
						continue
					}
					fmt.Fprintf(&buf, "(%d) %s\n", j, sd)
				}
			}
			tt.CheckStringEqual(buf.String(), test.details)
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
		p.Printf("-- this is %s's\nmulti-line payload", e.msg)
	}
	return e.cause
}
