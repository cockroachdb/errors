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

package errutil_test

import (
	goErr "errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/cockroachdb/errors/errbase"
	"github.com/cockroachdb/errors/errutil"
	"github.com/cockroachdb/errors/testutils"
)

func TestFormat(t *testing.T) {
	tt := testutils.T{t}

	testCases := []struct {
		name          string
		err           error
		expFmtSimple  string
		expFmtVerbose string
	}{
		{"fmt wrap + local msg + fmt leaf",
			&werrFmt{
				errutil.WithMessage(
					goErr.New("woo"), "waa"),
				"wuu"},
			`wuu: waa: woo`, `
wuu:
    -- verbose wrapper:
    wuu
  - waa:
  - woo`,
		},

		{"newf",
			errutil.Newf("waa: %s", "hello"),
			`waa: hello`, `
error with attached stack trace:
    github.com/cockroachdb/errors/errutil_test.TestFormat
    <tab><path>
    testing.tRunner
    <tab><path>
    runtime.goexit
    <tab><path>
  - error with embedded safe details: waa: %s
    -- arg 1: <string>
  - waa: hello`,
		},

		{"newf-empty",
			errutil.Newf(emptyString),
			``, `
error with attached stack trace:
    github.com/cockroachdb/errors/errutil_test.TestFormat
    <tab><path>
    testing.tRunner
    <tab><path>
    runtime.goexit
    <tab><path>
  - `,
		},

		{"newf-empty-arg",
			errutil.Newf(emptyString, 123),
			`%!(EXTRA int=123)`, `
error with attached stack trace:
    github.com/cockroachdb/errors/errutil_test.TestFormat
    <tab><path>
    testing.tRunner
    <tab><path>
    runtime.goexit
    <tab><path>
  - error with embedded safe details:
    -- arg 1: <int>
  - %!(EXTRA int=123)`,
		},

		{"wrapf",
			errutil.Wrapf(goErr.New("woo"), "waa: %s", "hello"),
			`waa: hello: woo`, `
error with attached stack trace:
    github.com/cockroachdb/errors/errutil_test.TestFormat
    <tab><path>
    testing.tRunner
    <tab><path>
    runtime.goexit
    <tab><path>
  - error with embedded safe details: waa: %s
    -- arg 1: <string>
  - waa: hello:
  - woo`,
		},

		{"wrapf-empty",
			errutil.Wrapf(goErr.New("woo"), emptyString),
			`woo`, `
error with attached stack trace:
    github.com/cockroachdb/errors/errutil_test.TestFormat
    <tab><path>
    testing.tRunner
    <tab><path>
    runtime.goexit
    <tab><path>
  - woo`,
		},

		{"wrapf-empty-arg",
			errutil.Wrapf(goErr.New("woo"), emptyString, 123),
			`%!(EXTRA int=123): woo`, `
error with attached stack trace:
    github.com/cockroachdb/errors/errutil_test.TestFormat
    <tab><path>
    testing.tRunner
    <tab><path>
    runtime.goexit
    <tab><path>
  - error with embedded safe details:
    -- arg 1: <int>
  - %!(EXTRA int=123):
  - woo`,
		},

		{"handled assert",
			errutil.HandleAsAssertionFailure(&werrFmt{goErr.New("woo"), "wuu"}),
			`wuu: woo`,
			`
assertion failure
  - error with attached stack trace:
    github.com/cockroachdb/errors/errutil_test.TestFormat
    <tab><path>
    testing.tRunner
    <tab><path>
    runtime.goexit
    <tab><path>
  - wuu: woo:
    original cause behind barrier: wuu:
        -- verbose wrapper:
        wuu
      - woo`,
		},

		{"assert + wrap",
			errutil.NewAssertionErrorWithWrappedErrf(&werrFmt{goErr.New("woo"), "wuu"}, "waa: %s", "hello"),
			`waa: hello: wuu: woo`, `
assertion failure
  - error with attached stack trace:
    github.com/cockroachdb/errors/errutil_test.TestFormat
    <tab><path>
    testing.tRunner
    <tab><path>
    runtime.goexit
    <tab><path>
  - error with embedded safe details: waa: %s
    -- arg 1: <string>
  - waa: hello:
  - wuu: woo:
    original cause behind barrier: wuu:
        -- verbose wrapper:
        wuu
      - woo`,
		},

		{"assert + wrap empty",
			errutil.NewAssertionErrorWithWrappedErrf(&werrFmt{goErr.New("woo"), "wuu"}, emptyString),
			`wuu: woo`, `
assertion failure
  - error with attached stack trace:
    github.com/cockroachdb/errors/errutil_test.TestFormat
    <tab><path>
    testing.tRunner
    <tab><path>
    runtime.goexit
    <tab><path>
  - wuu: woo:
    original cause behind barrier: wuu:
        -- verbose wrapper:
        wuu
      - woo`,
		},

		{"assert + wrap empty+arg",
			errutil.NewAssertionErrorWithWrappedErrf(&werrFmt{goErr.New("woo"), "wuu"}, emptyString, 123),
			`%!(EXTRA int=123): wuu: woo`, `
assertion failure
  - error with attached stack trace:
    github.com/cockroachdb/errors/errutil_test.TestFormat
    <tab><path>
    testing.tRunner
    <tab><path>
    runtime.goexit
    <tab><path>
  - error with embedded safe details:
    -- arg 1: <int>
  - %!(EXTRA int=123):
  - wuu: woo:
    original cause behind barrier: wuu:
        -- verbose wrapper:
        wuu
      - woo`,
		},
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
			spv = fileref.ReplaceAllString(spv, "<path>")
			spv = strings.ReplaceAll(spv, "\t", "<tab>")
			tt.CheckEqual(spv, refV)
		})
	}
}

var fileref = regexp.MustCompile(`([a-zA-Z0-9\._/-]*\.(?:go|s):\d+)`)

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

var emptyString = ""
