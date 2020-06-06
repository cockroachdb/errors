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

package withstack_test

import (
	"context"
	"errors"
	goErr "errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/interspace/errors/errbase"
	"github.com/interspace/errors/markers"
	"github.com/interspace/errors/testutils"
	"github.com/interspace/errors/withstack"
	"github.com/kr/pretty"
)

func TestWithStack(t *testing.T) {
	tt := testutils.T{T: t}

	origErr := withstack.WithStack(errors.New("hello"))
	t.Logf("start err: %# v", pretty.Formatter(origErr))

	// Show that there is indeed a stack trace.
	s, ok := origErr.(errbase.SafeDetailer)
	if !ok {
		t.Errorf("unexpected: error does not implement SafeDetailer")
	} else {
		details := s.SafeDetails()
		tt.Check(len(details) > 0 && strings.Contains(details[0], "withstack_test.go"))
	}

	enc := errbase.EncodeError(context.Background(), origErr)
	newErr := errbase.DecodeError(context.Background(), enc)

	// In any case, the library preserves the error message.
	tt.CheckStringEqual(newErr.Error(), origErr.Error())

	// The decoded error is marker-equal with the original one.
	tt.Check(markers.Is(newErr, origErr))

	// Also the new error includes the stack trace.
	s, ok = newErr.(errbase.SafeDetailer)
	if !ok {
		t.Errorf("unexpected: error does not implement SafeDetailer")
	} else {
		details := s.SafeDetails()
		tt.Check(len(details) > 0 && strings.Contains(details[0], "withstack_test.go"))
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
		{"withstack",
			withstack.WithStack(baseErr),
			woo, `
woo
(1) attached stack trace
  | github.com/interspace/errors/withstack_test.TestFormat
  | <tab><path>
  | testing.tRunner
  | <tab><path>
  | runtime.goexit
  | <tab><path>
Wraps: (2) woo
Error types: (1) *withstack.withStack (2) *errors.errorString`},

		{"withstack + wrapper",
			withstack.WithStack(&werrFmt{baseErr, "waa"}),
			waawoo, `
waa: woo
(1) attached stack trace
  | github.com/interspace/errors/withstack_test.TestFormat
  | <tab><path>
  | testing.tRunner
  | <tab><path>
  | runtime.goexit
  | <tab><path>
Wraps: (2) waa
  | -- verbose wrapper:
  | waa
Wraps: (3) woo
Error types: (1) *withstack.withStack (2) *withstack_test.werrFmt (3) *errors.errorString`},

		{"wrapper + withstack",
			&werrFmt{withstack.WithStack(baseErr), "waa"},
			waawoo, `
waa: woo
(1) waa
  | -- verbose wrapper:
  | waa
Wraps: (2) attached stack trace
  | github.com/interspace/errors/withstack_test.TestFormat
  | <tab><path>
  | testing.tRunner
  | <tab><path>
  | runtime.goexit
  | <tab><path>
Wraps: (3) woo
Error types: (1) *withstack_test.werrFmt (2) *withstack.withStack (3) *errors.errorString`},

		{"withstack + withstack",
			withstack.WithStack(
				withstack.WithStack(baseErr)),
			woo, `
woo
(1) attached stack trace
  | github.com/interspace/errors/withstack_test.TestFormat
  | <tab><path>
  | [...repeated from below...]
Wraps: (2) attached stack trace
  | github.com/interspace/errors/withstack_test.TestFormat
  | <tab><path>
  | testing.tRunner
  | <tab><path>
  | runtime.goexit
  | <tab><path>
Wraps: (3) woo
Error types: (1) *withstack.withStack (2) *withstack.withStack (3) *errors.errorString`},
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
			spv = fileref.ReplaceAllString(spv, "<path>")
			// spv = funref.ReplaceAllString(spv, "<path>/${fun}")
			spv = strings.ReplaceAll(spv, "\t", "<tab>")
			tt.CheckStringEqual(spv, refV)
		})
	}
}

var fileref = regexp.MustCompile(`([a-zA-Z0-9\._/@-]*\.(?:go|s):\d+)`)

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
