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
wuu: waa: woo
- (*errors.errorString:) woo
- (*errutil.withMessage:) waa
- (*errutil_test.werrFmt:) wuu
    -- this is wuu's
    multi-line payload`,
		},

		{"newf",
			errutil.Newf("waa: %s", "hello"),
			`waa: hello`, `
waa: hello
- (*errors.errorString:) waa: hello
- (*safedetails.withSafeDetails:) 2 details enclosed
- (*withstack.withStack:)
    github.com/cockroachdb/errors/errutil_test.TestFormat
    <tab><path>
    testing.tRunner
    <tab><path>
    runtime.goexit
    <tab><path>`,
		},

		{"newf-empty",
			errutil.Newf(emptyString),
			``, `

- (*errors.errorString:)
- (*withstack.withStack:)
    github.com/cockroachdb/errors/errutil_test.TestFormat
    <tab><path>
    testing.tRunner
    <tab><path>
    runtime.goexit
    <tab><path>`,
		},

		{"newf-empty-arg",
			errutil.Newf(emptyString, 123),
			`%!(EXTRA int=123)`, `
%!(EXTRA int=123)
- (*errors.errorString:) %!(EXTRA int=123)
- (*safedetails.withSafeDetails:) 2 details enclosed
- (*withstack.withStack:)
    github.com/cockroachdb/errors/errutil_test.TestFormat
    <tab><path>
    testing.tRunner
    <tab><path>
    runtime.goexit
    <tab><path>`,
		},

		{"wrapf",
			errutil.Wrapf(goErr.New("woo"), "waa: %s", "hello"),
			`waa: hello: woo`, `
waa: hello: woo
- (*errors.errorString:) woo
- (*errutil.withMessage:) waa: hello
- (*safedetails.withSafeDetails:) 2 details enclosed
- (*withstack.withStack:)
    github.com/cockroachdb/errors/errutil_test.TestFormat
    <tab><path>
    testing.tRunner
    <tab><path>
    runtime.goexit
    <tab><path>`,
		},

		{"wrapf-empty",
			errutil.Wrapf(goErr.New("woo"), emptyString),
			`woo`, `
woo
- (*errors.errorString:) woo
- (*withstack.withStack:)
    github.com/cockroachdb/errors/errutil_test.TestFormat
    <tab><path>
    testing.tRunner
    <tab><path>
    runtime.goexit
    <tab><path>`,
		},

		{"wrapf-empty-arg",
			errutil.Wrapf(goErr.New("woo"), emptyString, 123),
			`%!(EXTRA int=123): woo`, `
%!(EXTRA int=123): woo
- (*errors.errorString:) woo
- (*errutil.withMessage:) %!(EXTRA int=123)
- (*safedetails.withSafeDetails:) 2 details enclosed
- (*withstack.withStack:)
    github.com/cockroachdb/errors/errutil_test.TestFormat
    <tab><path>
    testing.tRunner
    <tab><path>
    runtime.goexit
    <tab><path>`,
		},

		{"handled assert",
			errutil.HandleAsAssertionFailure(&werrFmt{goErr.New("woo"), "wuu"}),
			`wuu: woo`,
			`
wuu: woo
- (*barriers.barrierError:) wuu: woo
    hidden cause: wuu: woo
    - (*errors.errorString:) woo
    - (*errutil_test.werrFmt:) wuu
        -- this is wuu's
        multi-line payload
- (*withstack.withStack:)
    github.com/cockroachdb/errors/errutil_test.TestFormat
    <tab><path>
    testing.tRunner
    <tab><path>
    runtime.goexit
    <tab><path>
- (*assert.withAssertionFailure:)`,
		},

		{"assert + wrap",
			errutil.NewAssertionErrorWithWrappedErrf(&werrFmt{goErr.New("woo"), "wuu"}, "waa: %s", "hello"),
			`waa: hello: wuu: woo`, `
waa: hello: wuu: woo
- (*barriers.barrierError:) wuu: woo
    hidden cause: wuu: woo
    - (*errors.errorString:) woo
    - (*errutil_test.werrFmt:) wuu
        -- this is wuu's
        multi-line payload
- (*errutil.withMessage:) waa: hello
- (*safedetails.withSafeDetails:) 2 details enclosed
- (*withstack.withStack:)
    github.com/cockroachdb/errors/errutil_test.TestFormat
    <tab><path>
    testing.tRunner
    <tab><path>
    runtime.goexit
    <tab><path>
- (*assert.withAssertionFailure:)`,
		},

		{"assert + wrap empty",
			errutil.NewAssertionErrorWithWrappedErrf(&werrFmt{goErr.New("woo"), "wuu"}, emptyString),
			`wuu: woo`, `
wuu: woo
- (*barriers.barrierError:) wuu: woo
    hidden cause: wuu: woo
    - (*errors.errorString:) woo
    - (*errutil_test.werrFmt:) wuu
        -- this is wuu's
        multi-line payload
- (*withstack.withStack:)
    github.com/cockroachdb/errors/errutil_test.TestFormat
    <tab><path>
    testing.tRunner
    <tab><path>
    runtime.goexit
    <tab><path>
- (*assert.withAssertionFailure:)`,
		},

		{"assert + wrap empty+arg",
			errutil.NewAssertionErrorWithWrappedErrf(&werrFmt{goErr.New("woo"), "wuu"}, emptyString, 123),
			`%!(EXTRA int=123): wuu: woo`, `
%!(EXTRA int=123): wuu: woo
- (*barriers.barrierError:) wuu: woo
    hidden cause: wuu: woo
    - (*errors.errorString:) woo
    - (*errutil_test.werrFmt:) wuu
        -- this is wuu's
        multi-line payload
- (*errutil.withMessage:) %!(EXTRA int=123)
- (*safedetails.withSafeDetails:) 2 details enclosed
- (*withstack.withStack:)
    github.com/cockroachdb/errors/errutil_test.TestFormat
    <tab><path>
    testing.tRunner
    <tab><path>
    runtime.goexit
    <tab><path>
- (*assert.withAssertionFailure:)`,
		},
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
		p.Printf("-- this is %s's\nmulti-line payload", e.msg)
	}
	return e.cause
}

var emptyString = ""
