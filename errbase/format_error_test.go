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

package errbase_test

import (
	"context"
	goErr "errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/cockroachdb/errors/errbase"
	"github.com/cockroachdb/errors/testutils"
	pkgErr "github.com/pkg/errors"
)

func TestSimplifyStacks(t *testing.T) {
	leaf := func() error {
		return pkgErr.New("hello world")
	}
	wrapper := func() error {
		err := leaf()
		return pkgErr.WithStack(err)
	}
	errWrapper := wrapper()
	t.Logf("error: %+v", errWrapper)

	t.Run("low level API", func(t *testing.T) {
		tt := testutils.T{t}
		// Extract the stack trace from the leaf.
		errLeaf := errbase.UnwrapOnce(errWrapper)
		leafP, ok := errLeaf.(errbase.StackTraceProvider)
		if !ok {
			t.Fatal("leaf error does not provide stack trace")
		}
		leafT := leafP.StackTrace()
		spv := fmtClean(leafT)
		t.Logf("-- leaf trace --%+v", spv)
		if !strings.Contains(spv, "TestSimplifyStacks") {
			t.Fatalf("expected test function in trace, got:%v", spv)
		}
		leafLines := strings.Split(spv, "\n")

		// Extract the stack trace from the wrapper.
		wrapperP, ok := errWrapper.(errbase.StackTraceProvider)
		if !ok {
			t.Fatal("wrapper error does not provide stack trace")
		}
		wrapperT := wrapperP.StackTrace()
		spv = fmtClean(wrapperT)
		t.Logf("-- wrapper trace --%+v", spv)
		wrapperLines := strings.Split(spv, "\n")

		// Sanity check before we verify the result.
		tt.Check(len(wrapperLines) > 0)
		tt.CheckDeepEqual(wrapperLines[3:], leafLines[5:])

		// Elide the suffix and verify that we arrive to the same result.
		simplified, hasElided := errbase.ElideSharedStackTraceSuffix(leafT, wrapperT)
		spv = fmtClean(simplified)
		t.Logf("-- simplified (%v) --%+v", hasElided, spv)
		simplifiedLines := strings.Split(spv, "\n")
		tt.CheckDeepEqual(simplifiedLines, wrapperLines[0:3])
	})

	t.Run("high level API", func(t *testing.T) {
		tt := testutils.T{t}

		spv := fmtClean(&errFormatter{errWrapper})
		tt.CheckStringEqual(spv, `hello world
(1)
  | github.com/cockroachdb/errors/errbase_test.TestSimplifyStacks.func2
  | <tab><path>:<lineno>
  | [...repeated from below...]
Wraps: (2) hello world
  | github.com/cockroachdb/errors/errbase_test.TestSimplifyStacks.func1
  | <tab><path>:<lineno>
  | github.com/cockroachdb/errors/errbase_test.TestSimplifyStacks.func2
  | <tab><path>:<lineno>
  | github.com/cockroachdb/errors/errbase_test.TestSimplifyStacks
  | <tab><path>:<lineno>
  | testing.tRunner
  | <tab><path>:<lineno>
  | runtime.goexit
  | <tab><path>:<lineno>
Error types: (1) *errors.withStack (2) *errors.fundamental`)
	})
}

type errFormatter struct{ err error }

func (f *errFormatter) Format(s fmt.State, verb rune) { errbase.FormatError(f.err, s, verb) }

func TestFormat(t *testing.T) {
	tt := testutils.T{t}

	ctx := context.Background()
	const woo = `woo`
	const waa = `waa`
	const mwoo = "woo\nother"
	const waawoo = `waa: woo`
	const wuuwaawoo = `wuu: waa: woo`
	testCases := []struct {
		name          string
		err           error
		expFmtSimple  string
		expFmtVerbose string
		// We're expecting the result of printing an error with %q
		// to be the quoted version of %s in the general case.
		// However, the implementation of (*pkgErr.withMessage).Format()
		// gets this wrong, so we need a separate ref string for this
		// specific case.
		expFmtQuote string
	}{
		{"nofmt leaf", &errNoFmt{woo}, woo, woo, ``},
		{"nofmt leaf multiline", &errNoFmt{mwoo}, mwoo, mwoo, ``},

		{"fmt-old leaf", &errFmto{woo}, woo, `
woo
-- this is woo's
multi-line payload`, ``,
		},

		{"fmt-old leaf multiline", &errFmto{mwoo}, mwoo, `
woo
other
-- this is woo
other's
multi-line payload`, ``,
		},

		{"fmt-partial leaf",
			&errFmtp{woo},
			woo, `
woo
(1) woo
Error types: (1) *errbase_test.errFmtp`, ``,
		},

		{"fmt-partial leaf multiline",
			&errFmtp{mwoo},
			mwoo, `
woo
(1) woo
  | other
Error types: (1) *errbase_test.errFmtp`, ``,
		},

		{"fmt leaf",
			&errFmt{woo},
			woo, `
woo
(1) woo
  | -- this is woo's
  | multi-line leaf payload
Error types: (1) *errbase_test.errFmt`, ``,
		},

		{"fmt leaf multiline",
			&errFmt{mwoo},
			mwoo, `
woo
(1) woo
  | other
  | -- this is woo
  | other's
  | multi-line leaf payload
Error types: (1) *errbase_test.errFmt`, ``,
		},

		{"nofmt leaf + nofmt wrap",
			&werrNoFmt{&errNoFmt{woo}, waa},
			waawoo, waawoo, ``},

		{"nofmt leaf + fmt-old wrap",
			&werrFmto{&errNoFmt{woo}, waa},
			waawoo, `
woo
-- this is waa's
multi-line payload (fmt)`, ``,
		},

		{"nofmt leaf + fmt-partial wrap",
			&werrFmtp{&errNoFmt{woo}, waa},
			waawoo, `
waa: woo
(1) waa
Wraps: (2) woo
Error types: (1) *errbase_test.werrFmtp (2) *errbase_test.errNoFmt`, ``,
		},

		{"nofmt leaf + fmt wrap",
			&werrFmt{&errNoFmt{woo}, waa},
			waawoo, `
waa: woo
(1) waa
  | -- this is waa's
  | multi-line wrapper payload
Wraps: (2) woo
Error types: (1) *errbase_test.werrFmt (2) *errbase_test.errNoFmt`, ``,
		},

		{"fmt-old leaf + nofmt wrap",
			&werrNoFmt{&errFmto{woo}, waa},
			waawoo, waawoo, ``},

		{"fmt-old leaf + fmt-old wrap",
			&werrFmto{&errFmto{woo}, waa},
			waawoo, `
woo
-- this is woo's
multi-line payload
-- this is waa's
multi-line payload (fmt)`, ``,
		},

		{"fmt-old leaf + fmt-partial wrap",
			&werrFmtp{&errFmto{woo}, waa},
			waawoo, `
waa: woo
(1) waa
Wraps: (2) woo
  | -- this is woo's
  | multi-line payload
Error types: (1) *errbase_test.werrFmtp (2) *errbase_test.errFmto`, ``,
		},

		{"fmt-old leaf + fmt wrap",
			&werrFmt{&errFmto{woo}, waa},
			waawoo, `
waa: woo
(1) waa
  | -- this is waa's
  | multi-line wrapper payload
Wraps: (2) woo
  | -- this is woo's
  | multi-line payload
Error types: (1) *errbase_test.werrFmt (2) *errbase_test.errFmto`, ``,
		},

		{"fmt-partial leaf + nofmt wrap",
			&werrNoFmt{&errFmtp{woo}, waa},
			waawoo, waawoo, ``},

		{"fmt-partial leaf + fmt-old wrap",
			&werrFmto{&errFmtp{woo}, waa},
			waawoo, `
woo
(1) woo
Error types: (1) *errbase_test.errFmtp
-- this is waa's
multi-line payload (fmt)`, ``,
		},

		{"fmt-partial leaf + fmt-partial wrap",
			&werrFmtp{&errFmtp{woo}, waa},
			waawoo, `
waa: woo
(1) waa
Wraps: (2) woo
  | (1) woo
  | Error types: (1) *errbase_test.errFmtp
Error types: (1) *errbase_test.werrFmtp (2) *errbase_test.errFmtp`, ``,
		},

		{"fmt-partial leaf + fmt wrap",
			&werrFmt{&errFmtp{woo}, waa},
			waawoo, `
waa: woo
(1) waa
  | -- this is waa's
  | multi-line wrapper payload
Wraps: (2) woo
  | (1) woo
  | Error types: (1) *errbase_test.errFmtp
Error types: (1) *errbase_test.werrFmt (2) *errbase_test.errFmtp`, ``,
		},

		{"fmt leaf + nofmt wrap",
			&werrNoFmt{&errFmt{woo}, waa},
			waawoo, waawoo, ``},

		{"fmt leaf + fmt-old wrap",
			&werrFmto{&errFmt{woo}, waa},
			waawoo, `
woo
(1) woo
  | -- this is woo's
  | multi-line leaf payload
Error types: (1) *errbase_test.errFmt
-- this is waa's
multi-line payload (fmt)`, ``,
		},

		{"fmt leaf + fmt-partial wrap",
			&werrFmtp{&errFmt{woo}, waa},
			waawoo, `
waa: woo
(1) waa
Wraps: (2) woo
  | -- this is woo's
  | multi-line leaf payload
Error types: (1) *errbase_test.werrFmtp (2) *errbase_test.errFmt`, ``,
		},

		{"fmt leaf + fmt wrap",
			&werrFmt{&errFmt{woo}, waa},
			waawoo, `
waa: woo
(1) waa
  | -- this is waa's
  | multi-line wrapper payload
Wraps: (2) woo
  | -- this is woo's
  | multi-line leaf payload
Error types: (1) *errbase_test.werrFmt (2) *errbase_test.errFmt`, ``,
		},

		{"nofmt wrap in + nofmt wrap out",
			&werrNoFmt{&werrNoFmt{&errFmt{woo}, waa}, "wuu"},
			wuuwaawoo, wuuwaawoo, ``},

		{"nofmt wrap in + fmd-old wrap out",
			&werrFmto{&werrNoFmt{&errFmt{woo}, waa}, "wuu"},
			wuuwaawoo, `
waa: woo
-- this is wuu's
multi-line payload (fmt)`, ``,
		},

		{"nofmt wrap in + fmt wrap out",
			&werrFmt{&werrNoFmt{&errFmt{woo}, waa}, "wuu"},
			wuuwaawoo, `
wuu: waa: woo
(1) wuu
  | -- this is wuu's
  | multi-line wrapper payload
Wraps: (2) waa
Wraps: (3) woo
  | -- this is woo's
  | multi-line leaf payload
Error types: (1) *errbase_test.werrFmt (2) *errbase_test.werrNoFmt (3) *errbase_test.errFmt`, ``,
		},

		{"fmt-old wrap in + nofmt wrap out",
			&werrNoFmt{&werrFmto{&errFmt{woo}, waa}, "wuu"},
			wuuwaawoo, wuuwaawoo, ``},

		{"fmt-old wrap in + fmd-old wrap out",
			&werrFmto{&werrFmto{&errFmt{woo}, waa}, "wuu"},
			wuuwaawoo, `
woo
(1) woo
  | -- this is woo's
  | multi-line leaf payload
Error types: (1) *errbase_test.errFmt
-- this is waa's
multi-line payload (fmt)
-- this is wuu's
multi-line payload (fmt)`, ``,
		},

		{"fmt-old wrap in + fmt wrap out",
			&werrFmt{&werrFmto{&errFmt{woo}, waa}, "wuu"},
			wuuwaawoo, `
wuu: waa: woo
(1) wuu
  | -- this is wuu's
  | multi-line wrapper payload
Wraps: (2) waa
Wraps: (3) woo
  | -- this is woo's
  | multi-line leaf payload
Error types: (1) *errbase_test.werrFmt (2) *errbase_test.werrFmto (3) *errbase_test.errFmt`, ``,
		},

		{"fmt wrap in + nofmt wrap out",
			&werrNoFmt{&werrFmt{&errFmt{woo}, waa}, "wuu"},
			wuuwaawoo, wuuwaawoo, ``},

		{"fmt wrap in + fmd-old wrap out",
			&werrFmto{&werrFmt{&errFmt{woo}, waa}, "wuu"},
			wuuwaawoo, `
waa: woo
(1) waa
  | -- this is waa's
  | multi-line wrapper payload
Wraps: (2) woo
  | -- this is woo's
  | multi-line leaf payload
Error types: (1) *errbase_test.werrFmt (2) *errbase_test.errFmt
-- this is wuu's
multi-line payload (fmt)`, ``,
		},

		{"fmt wrap in + fmt wrap out",
			&werrFmt{&werrFmt{&errFmt{woo}, waa}, "wuu"},
			wuuwaawoo, `
wuu: waa: woo
(1) wuu
  | -- this is wuu's
  | multi-line wrapper payload
Wraps: (2) waa
  | -- this is waa's
  | multi-line wrapper payload
Wraps: (3) woo
  | -- this is woo's
  | multi-line leaf payload
Error types: (1) *errbase_test.werrFmt (2) *errbase_test.werrFmt (3) *errbase_test.errFmt`, ``,
		},

		// Opaque leaf.
		{"opaque leaf",
			errbase.DecodeError(ctx, errbase.EncodeError(ctx, &errNoFmt{woo})),
			woo, `
woo
(1) woo
  |
  | (opaque error leaf)
  | type name: github.com/cockroachdb/errors/errbase_test/*errbase_test.errNoFmt
Error types: (1) *errbase.opaqueLeaf`, ``},

		// Opaque wrapper.
		{"opaque wrapper",
			errbase.DecodeError(ctx, errbase.EncodeError(ctx, &werrNoFmt{goErr.New(woo), waa})),
			waawoo, `
waa: woo
(1) waa
  |
  | (opaque error wrapper)
  | type name: github.com/cockroachdb/errors/errbase_test/*errbase_test.werrNoFmt
Wraps: (2) woo
Error types: (1) *errbase.opaqueWrapper (2) *errors.errorString`, ``},

		{"opaque wrapper+wrapper",
			errbase.DecodeError(ctx, errbase.EncodeError(ctx, &werrNoFmt{&werrNoFmt{goErr.New(woo), waa}, "wuu"})),
			wuuwaawoo, `
wuu: waa: woo
(1) wuu
  |
  | (opaque error wrapper)
  | type name: github.com/cockroachdb/errors/errbase_test/*errbase_test.werrNoFmt
Wraps: (2) waa
  |
  | (opaque error wrapper)
  | type name: github.com/cockroachdb/errors/errbase_test/*errbase_test.werrNoFmt
Wraps: (3) woo
Error types: (1) *errbase.opaqueWrapper (2) *errbase.opaqueWrapper (3) *errors.errorString`, ``},

		// Compatibility with github.com/pkg/errors.

		{"pkg msg + fmt leaf",
			pkgErr.WithMessage(&errFmt{woo}, waa),
			waawoo, `
woo
(1) woo
  | -- this is woo's
  | multi-line leaf payload
Error types: (1) *errbase_test.errFmt
waa`,
			// The implementation of (*pkgErr.withMessage).Format() is wrong for %q. Oh well...
			`waa: woo`,
		},

		{"fmt wrap + pkg msg + fmt leaf",
			&werrFmt{pkgErr.WithMessage(&errFmt{woo}, waa), "wuu"},
			wuuwaawoo, `
wuu: waa: woo
(1) wuu
  | -- this is wuu's
  | multi-line wrapper payload
Wraps: (2) waa
Wraps: (3) woo
  | -- this is woo's
  | multi-line leaf payload
Error types: (1) *errbase_test.werrFmt (2) *errors.withMessage (3) *errbase_test.errFmt`, ``,
		},

		{"fmt wrap + pkg msg1 + pkg.msg2 + fmt leaf",
			&werrFmt{
				pkgErr.WithMessage(
					pkgErr.WithMessage(
						&errFmt{woo}, "waa2"),
					"waa1"),
				"wuu"},
			`wuu: waa1: waa2: woo`, `
wuu: waa1: waa2: woo
(1) wuu
  | -- this is wuu's
  | multi-line wrapper payload
Wraps: (2) waa1
Wraps: (3) waa2
Wraps: (4) woo
  | -- this is woo's
  | multi-line leaf payload
Error types: (1) *errbase_test.werrFmt (2) *errors.withMessage (3) *errors.withMessage (4) *errbase_test.errFmt`, ``,
		},

		{"fmt wrap + pkg stack + fmt leaf",
			&werrFmt{pkgErr.WithStack(&errFmt{woo}), waa},
			waawoo, `
waa: woo
(1) waa
  | -- this is waa's
  | multi-line wrapper payload
Wraps: (2)
  | github.com/cockroachdb/errors/errbase_test.TestFormat
  | <tab><path>:<lineno>
  | testing.tRunner
  | <tab><path>:<lineno>
  | runtime.goexit
  | <tab><path>:<lineno>
Wraps: (3) woo
  | -- this is woo's
  | multi-line leaf payload
Error types: (1) *errbase_test.werrFmt (2) *errors.withStack (3) *errbase_test.errFmt`, ``,
		},

		{"delegating wrap",
			&werrDelegate{&errNoFmt{woo}}, "prefix: woo", `
prefix: woo
(1) prefix
  | -- multi-line
  | wrapper payload
Wraps: (2) woo
Error types: (1) *errbase_test.werrDelegate (2) *errbase_test.errNoFmt`, ``,
		},

		{"delegating wrap + pkg stack + fmt leaf",
			&werrDelegate{pkgErr.WithStack(&errFmt{woo})},
			"prefix: woo", `
prefix: woo
(1) prefix
  | -- multi-line
  | wrapper payload
Wraps: (2)
  | github.com/cockroachdb/errors/errbase_test.TestFormat
  | <tab><path>:<lineno>
  | testing.tRunner
  | <tab><path>:<lineno>
  | runtime.goexit
  | <tab><path>:<lineno>
Wraps: (3) woo
  | -- this is woo's
  | multi-line leaf payload
Error types: (1) *errbase_test.werrDelegate (2) *errors.withStack (3) *errbase_test.errFmt`, ``,
		},

		{"empty wrap",
			&werrEmpty{&errNoFmt{woo}}, woo, `
woo
(1)
Wraps: (2) woo
Error types: (1) *errbase_test.werrEmpty (2) *errbase_test.errNoFmt`, ``,
		},

		{"empty wrap + pkg stack + fmt leaf",
			&werrEmpty{pkgErr.WithStack(&errFmt{woo})},
			woo, `
woo
(1)
Wraps: (2)
  | github.com/cockroachdb/errors/errbase_test.TestFormat
  | <tab><path>:<lineno>
  | testing.tRunner
  | <tab><path>:<lineno>
  | runtime.goexit
  | <tab><path>:<lineno>
Wraps: (3) woo
  | -- this is woo's
  | multi-line leaf payload
Error types: (1) *errbase_test.werrEmpty (2) *errors.withStack (3) *errbase_test.errFmt`, ``,
		},

		{"empty delegate wrap",
			&werrDelegateEmpty{&errNoFmt{woo}}, woo, `
woo
(1)
Wraps: (2) woo
Error types: (1) *errbase_test.werrDelegateEmpty (2) *errbase_test.errNoFmt`, ``,
		},

		{"empty delegate wrap + pkg stack + fmt leaf",
			&werrDelegateEmpty{pkgErr.WithStack(&errFmt{woo})},
			woo, `
woo
(1)
Wraps: (2)
  | github.com/cockroachdb/errors/errbase_test.TestFormat
  | <tab><path>:<lineno>
  | testing.tRunner
  | <tab><path>:<lineno>
  | runtime.goexit
  | <tab><path>:<lineno>
Wraps: (3) woo
  | -- this is woo's
  | multi-line leaf payload
Error types: (1) *errbase_test.werrDelegateEmpty (2) *errors.withStack (3) *errbase_test.errFmt`, ``,
		},

		{"delegating wrap noprefix + details",
			&werrDelegateNoPrefix{&errNoFmt{woo}}, woo, `
woo
(1) detail
Wraps: (2) woo
Error types: (1) *errbase_test.werrDelegateNoPrefix (2) *errbase_test.errNoFmt`, ``,
		},

		{"wrapper with truncated short msg",
			&werrWithElidedCause{&errNoFmt{woo}},
			"overridden message", `overridden message
(1) overridden message
Wraps: (2) woo
Error types: (1) *errbase_test.werrWithElidedCause (2) *errbase_test.errNoFmt`, ``},
	}

	for _, test := range testCases {
		tt.Run(test.name, func(tt testutils.T) {
			err := test.err

			// %s is simple formatting
			tt.CheckStringEqual(fmt.Sprintf("%s", err), test.expFmtSimple)

			// %v is simple formatting too, for compatibility with the past.
			tt.CheckStringEqual(fmt.Sprintf("%v", err), test.expFmtSimple)

			// %q is always like %s but quotes the result.
			ref := test.expFmtQuote
			if ref == "" {
				ref = fmt.Sprintf("%q", test.expFmtSimple)
			}
			tt.CheckStringEqual(fmt.Sprintf("%q", err), ref)

			// %+v is the verbose mode.
			refV := strings.TrimPrefix(test.expFmtVerbose, "\n")
			spv := fmtClean(err)
			tt.CheckStringEqual(spv, refV)
		})
	}
}

func fmtClean(x interface{}) string {
	spv := fmt.Sprintf("%+v", x)
	spv = fileref.ReplaceAllString(spv, "<path>:<lineno>")
	spv = strings.ReplaceAll(spv, "\t", "<tab>")
	return spv
}

var fileref = regexp.MustCompile(`([a-zA-Z0-9\._/@-]*\.(?:go|s):\d+)`)

// errNoFmt does neither implement Format() nor FormatError().
type errNoFmt struct{ msg string }

func (e *errNoFmt) Error() string { return e.msg }

// werrNoFmt does like errNoFmt. This is used to check
// that FormatError() knows about "dumb" error wrappers.
type werrNoFmt struct {
	cause error
	msg   string
}

func (e *werrNoFmt) Error() string { return fmt.Sprintf("%s: %v", e.msg, e.cause) }
func (e *werrNoFmt) Unwrap() error { return e.cause }

// errFmto has just a Format() method that does everything, and does
// not know about neither FormatError() nor errors.Formatter.
// This is the "old style" support for formatting, e.g. used
// in github.com/pkg/errors.
type errFmto struct{ msg string }

func (e *errFmto) Error() string { return e.msg }
func (e *errFmto) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			fmt.Fprint(s, e.msg)
			fmt.Fprintf(s, "\n-- this is %s's\nmulti-line payload", e.msg)
			return
		}
		fallthrough
	case 's', 'q':
		fmt.Fprintf(s, fmt.Sprintf("%%%s%c", flags(s), verb), e.msg)
	}
}

// errFmtoDelegate is like errFmto but the Error() method delegates to
// Format().
type errFmtoDelegate struct{ msg string }

func (e *errFmtoDelegate) Error() string { return fmt.Sprintf("%v", e) }
func (e *errFmtoDelegate) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			fmt.Fprint(s, e.msg)
			fmt.Fprintf(s, "\n-- this is %s's\nmulti-line payload", e.msg)
			return
		}
		fallthrough
	case 's', 'q':
		fmt.Fprintf(s, fmt.Sprintf("%%%s%c", flags(s), verb), e.msg)
	}
}

// werrFmto is like errFmto but is a wrapper.
type werrFmto struct {
	cause error
	msg   string
}

func (e *werrFmto) Error() string { return fmt.Sprintf("%s: %v", e.msg, e.cause) }
func (e *werrFmto) Unwrap() error { return e.cause }
func (e *werrFmto) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			fmt.Fprintf(s, "%+v", e.cause)
			fmt.Fprintf(s, "\n-- this is %s's\nmulti-line payload (fmt)", e.msg)
			return
		}
		fallthrough
	case 's', 'q':
		fmt.Fprintf(s, fmt.Sprintf("%%%s%c", flags(s), verb), e.Error())
	}
}

// werrFmtoDelegate is like errFmtoDelegate but is a wrapper.
type werrFmtoDelegate struct {
	cause error
	msg   string
}

func (e *werrFmtoDelegate) Error() string { return fmt.Sprintf("%v", e) }
func (e *werrFmtoDelegate) Unwrap() error { return e.cause }
func (e *werrFmtoDelegate) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			fmt.Fprintf(s, "%+v", e.cause)
			fmt.Fprint(s, "\n-- multi-line\npayload (fmt)")
			return
		}
		fallthrough
	case 's', 'q':
		fmts := fmt.Sprintf("%%%s%c", flags(s), verb)
		fmt.Fprintf(s, fmts, e.msg+": "+e.cause.Error())
	}
}

// errFmtp implements Format() that forwards to FormatError(),
// but does not implement errors.Formatter. It is used
// to check that FormatError() does the right thing.
type errFmtp struct {
	msg string
}

func (e *errFmtp) Error() string                 { return e.msg }
func (e *errFmtp) Format(s fmt.State, verb rune) { errbase.FormatError(e, s, verb) }

// werrFmtp is like errFmtp but is a wrapper.
type werrFmtp struct {
	cause error
	msg   string
}

func (e *werrFmtp) Error() string                 { return fmt.Sprintf("%s: %v", e.msg, e.cause) }
func (e *werrFmtp) Unwrap() error                 { return e.cause }
func (e *werrFmtp) Format(s fmt.State, verb rune) { errbase.FormatError(e, s, verb) }

// errFmt has both Format() and FormatError(),
// and demonstrates the common case of "rich" errors.
type errFmt struct{ msg string }

func (e *errFmt) Error() string                 { return e.msg }
func (e *errFmt) Format(s fmt.State, verb rune) { errbase.FormatError(e, s, verb) }
func (e *errFmt) FormatError(p errbase.Printer) error {
	p.Print(e.msg)
	if p.Detail() {
		p.Printf("-- this is %s's\nmulti-line leaf payload", e.msg)
	}
	return nil
}

// werrFmt is like errFmt but is a wrapper.
type werrFmt struct {
	cause error
	msg   string
}

func (e *werrFmt) Error() string                 { return fmt.Sprintf("%s: %v", e.msg, e.cause) }
func (e *werrFmt) Unwrap() error                 { return e.cause }
func (e *werrFmt) Format(s fmt.State, verb rune) { errbase.FormatError(e, s, verb) }
func (e *werrFmt) FormatError(p errbase.Printer) error {
	p.Print(e.msg)
	if p.Detail() {
		p.Printf("-- this is %s's\nmulti-line wrapper payload", e.msg)
	}
	return e.cause
}

func flags(s fmt.State) string {
	flags := ""
	if s.Flag('#') {
		flags += "#"
	}
	if s.Flag('+') {
		flags += "+"
	}
	if s.Flag(' ') {
		flags += " "
	}
	return flags
}

// TestDelegateProtocol checks that there is no infinite recursion
// when Error() delegates its behavior to FormatError().
func TestDelegateProtocol(t *testing.T) {
	tt := testutils.T{t}

	var err error
	err = &werrDelegate{&errNoFmt{"woo"}}
	tt.CheckStringEqual(fmt.Sprintf("%v", err), "prefix: woo")

	err = &werrDelegateNoPrefix{&errNoFmt{"woo"}}
	tt.CheckStringEqual(fmt.Sprintf("%v", err), "woo")
}

// werrDelegate delegates its Error() behavior to FormatError().
type werrDelegate struct {
	wrapped error
}

var _ fmt.Formatter = (*werrDelegate)(nil)
var _ errbase.Formatter = (*werrDelegate)(nil)

func (e *werrDelegate) Error() string                 { return fmt.Sprintf("%v", e) }
func (e *werrDelegate) Cause() error                  { return e.wrapped }
func (e *werrDelegate) Format(s fmt.State, verb rune) { errbase.FormatError(e, s, verb) }
func (e *werrDelegate) FormatError(p errbase.Printer) error {
	p.Print("prefix")
	if p.Detail() {
		p.Print("-- multi-line\nwrapper payload")
	}
	return e.wrapped
}

// werrDelegateNoPrefix delegates its Error() behavior to FormatError()
// via fmt.Format, has no prefix of its own in its short message
// but has a detail field.
type werrDelegateNoPrefix struct {
	wrapped error
}

var _ errbase.Formatter = (*werrDelegateNoPrefix)(nil)
var _ fmt.Formatter = (*werrDelegateNoPrefix)(nil)

func (e *werrDelegateNoPrefix) Error() string                 { return fmt.Sprintf("%v", e) }
func (e *werrDelegateNoPrefix) Cause() error                  { return e.wrapped }
func (e *werrDelegateNoPrefix) Format(s fmt.State, verb rune) { errbase.FormatError(e, s, verb) }
func (e *werrDelegateNoPrefix) FormatError(p errbase.Printer) error {
	if p.Detail() {
		p.Print("detail")
	}
	return e.wrapped
}

type werrDelegateEmpty struct {
	wrapped error
}

var _ errbase.Formatter = (*werrDelegateEmpty)(nil)
var _ fmt.Formatter = (*werrDelegateEmpty)(nil)

func (e *werrDelegateEmpty) Error() string                 { return fmt.Sprintf("%v", e) }
func (e *werrDelegateEmpty) Cause() error                  { return e.wrapped }
func (e *werrDelegateEmpty) Format(s fmt.State, verb rune) { errbase.FormatError(e, s, verb) }
func (e *werrDelegateEmpty) FormatError(p errbase.Printer) error {
	return e.wrapped
}

// werrEmpty has no message of its own.
type werrEmpty struct {
	wrapped error
}

var _ error = (*werrEmpty)(nil)
var _ fmt.Formatter = (*werrEmpty)(nil)

func (e *werrEmpty) Error() string                 { return e.wrapped.Error() }
func (e *werrEmpty) Cause() error                  { return e.wrapped }
func (e *werrEmpty) Format(s fmt.State, verb rune) { errbase.FormatError(e, s, verb) }

// werrWithElidedClause overrides its cause's Error() from its own
// short message.
type werrWithElidedCause struct {
	wrapped error
}

func (e *werrWithElidedCause) Error() string                 { return fmt.Sprintf("%v", e) }
func (e *werrWithElidedCause) Cause() error                  { return e.wrapped }
func (e *werrWithElidedCause) Format(s fmt.State, verb rune) { errbase.FormatError(e, s, verb) }
func (e *werrWithElidedCause) FormatError(p errbase.Printer) error {
	p.Print("overridden message")
	return nil
}
