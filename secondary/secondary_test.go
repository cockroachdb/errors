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

package secondary_test

import (
	"context"
	goErr "errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cockroachdb/errors/errbase"
	"github.com/cockroachdb/errors/markers"
	"github.com/cockroachdb/errors/secondary"
	"github.com/cockroachdb/errors/testutils"
	"github.com/kr/pretty"
	"github.com/pkg/errors"
)

// This test demonstrates that a secondary error annotation
// does not reveal the secondary error as a cause.
func TestHideSecondaryError(t *testing.T) {
	tt := testutils.T{T: t}

	origErr := errors.New("hello")
	err1 := errors.Wrap(origErr, "world")

	// Assertion: without the annotation, the cause can be identified.
	tt.Assert(markers.Is(err1, origErr))

	// This test: the secondary error is not visible as cause.
	err := secondary.WithSecondaryError(errors.New("other"), err1)
	tt.Check(!markers.Is(err, origErr))
}

// This test demonstrates that the secondary error details
// are preserved, even when they go through the network.
func TestSecondaryErrorMaskedDetails(t *testing.T) {
	tt := testutils.T{T: t}

	origErr := errors.New("hello original")

	b := secondary.WithSecondaryError(errors.New("message hidden"), origErr)

	// Assertion: the original message is hidden.
	tt.Assert(!strings.Contains(b.Error(), "original"))

	// This test: the details are available when printing the error details.
	errV := fmt.Sprintf("%+v", b)
	tt.Check(strings.Contains(errV, "original"))

	// Simulate a network traversal.
	enc := errbase.EncodeError(context.Background(), b)
	newB := errbase.DecodeError(context.Background(), enc)

	t.Logf("decoded: %# v", pretty.Formatter(newB))

	// The original message is hidden.
	tt.Check(!strings.Contains(b.Error(), "original"))

	// The cause is still hidden.
	tt.Check(!markers.Is(newB, origErr))

	// However the cause's details are still visible.
	errV = fmt.Sprintf("%+v", newB)
	tt.Check(strings.Contains(errV, "original"))
}

// This test demonstrates how CombineErrors preserves both errors
// regardless of whether either is nil.
func TestCombineErrors(t *testing.T) {
	tt := testutils.T{T: t}
	err1 := errors.New("err1")
	err2 := errors.New("err2")

	testData := []struct {
		errA error
		errB error
		errC error
	}{
		{nil, nil, nil},
		{err1, nil, err1},
		{nil, err2, err2},
		{err1, err2, secondary.WithSecondaryError(err1, err2)},
	}

	for _, test := range testData {
		err := secondary.CombineErrors(test.errA, test.errB)
		tt.CheckDeepEqual(err, test.errC)
	}
}

func TestFormat(t *testing.T) {
	tt := testutils.T{t}

	baseErr := goErr.New("woo")
	const woo = `woo`
	const waawoo = `waa: woo`
	testCases := []struct {
		name          string
		err           error
		expFmtSimple  string
		expFmtVerbose string
	}{
		{"sec",
			secondary.WithSecondaryError(baseErr, goErr.New("wuu")),
			woo, `
combined error
    ancillary error: wuu
    (main error follows)
  - woo`},

		{"sec+sec chain",
			secondary.WithSecondaryError(
				secondary.WithSecondaryError(baseErr,
					goErr.New("payload1")),
				goErr.New("payload2")),
			woo, `
combined error
    ancillary error: payload2
    (main error follows)
  - combined error
    ancillary error: payload1
    (main error follows)
  - woo`},

		{"sec+sec nested",
			secondary.WithSecondaryError(baseErr,
				secondary.WithSecondaryError(
					goErr.New("payload1"), goErr.New("payload2"))),
			woo, `
combined error
    ancillary error: combined error
        ancillary error: payload2
        (main error follows)
      - payload1
    (main error follows)
  - woo`},

		{"sec + wrapper chain",
			secondary.WithSecondaryError(&werrFmt{baseErr, "waa"},
				goErr.New("wuu")),
			waawoo, `
combined error
    ancillary error: wuu
    (main error follows)
  - waa:
    -- verbose wrapper:
    waa
  - woo`},

		{"sec + wrapper nested",
			secondary.WithSecondaryError(baseErr,
				&werrFmt{goErr.New("wuu"), "waa"}),
			woo, `
combined error
    ancillary error: waa:
        -- verbose wrapper:
        waa
      - wuu
    (main error follows)
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
