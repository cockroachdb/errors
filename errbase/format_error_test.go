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

func TestFormat(t *testing.T) {
	tt := testutils.T{t}

	ctx := context.Background()
	const woo = `woo`
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
		{"nofmt leaf", &errNoFmt{"woo"}, woo, woo, ``},

		{"fmt-old leaf",
			&errFmto{"woo"},
			woo, `
woo
-- verbose leaf (fmt):
woo`, ``,
		},

		{"fmt-partial leaf",
			&errFmtp{"woo"},
			woo, woo, ``,
		},

		{"fmt leaf",
			&errFmt{"woo"},
			woo, `
woo:
    -- verbose leaf:
    woo`, ``,
		},

		{"nofmt leaf + nofmt wrap",
			&werrNoFmt{&errNoFmt{"woo"}, "waa"},
			waawoo, waawoo, ``},

		{"nofmt leaf + fmt-old wrap",
			&werrFmto{&errNoFmt{"woo"}, "waa"},
			waawoo, `
woo
-- verbose wrapper (fmt):
waa`, ``,
		},

		{"nofmt leaf + fmt-partial wrap",
			&werrFmtp{&errNoFmt{"woo"}, "waa"},
			waawoo, `
waa:
  - woo`, ``,
		},

		{"nofmt leaf + fmt wrap",
			&werrFmt{&errNoFmt{"woo"}, "waa"},
			waawoo, `
waa:
    -- verbose wrapper:
    waa
  - woo`, ``,
		},

		{"fmt-old leaf + nofmt wrap",
			&werrNoFmt{&errFmto{"woo"}, "waa"},
			waawoo, waawoo, ``},

		{"fmt-old leaf + fmt-old wrap",
			&werrFmto{&errFmto{"woo"}, "waa"},
			waawoo, `
woo
-- verbose leaf (fmt):
woo
-- verbose wrapper (fmt):
waa`, ``,
		},

		{"fmt-old leaf + fmt-partial wrap",
			&werrFmtp{&errFmto{"woo"}, "waa"},
			waawoo, `
waa:
  - woo
    -- verbose leaf (fmt):
    woo`, ``,
		},

		{"fmt-old leaf + fmt wrap",
			&werrFmt{&errFmto{"woo"}, "waa"},
			waawoo, `
waa:
    -- verbose wrapper:
    waa
  - woo
    -- verbose leaf (fmt):
    woo`, ``,
		},

		{"fmt-partial leaf + nofmt wrap",
			&werrNoFmt{&errFmtp{"woo"}, "waa"},
			waawoo, waawoo, ``},

		{"fmt-partial leaf + fmt-old wrap",
			&werrFmto{&errFmtp{"woo"}, "waa"},
			waawoo, `
woo
-- verbose wrapper (fmt):
waa`, ``,
		},

		{"fmt-partial leaf + fmt-partial wrap",
			&werrFmtp{&errFmtp{"woo"}, "waa"},
			waawoo, `
waa:
  - woo`, ``,
		},

		{"fmt-partial leaf + fmt wrap",
			&werrFmt{&errFmtp{"woo"}, "waa"},
			waawoo, `
waa:
    -- verbose wrapper:
    waa
  - woo`, ``,
		},

		{"fmt leaf + nofmt wrap",
			&werrNoFmt{&errFmt{"woo"}, "waa"},
			waawoo, waawoo, ``},

		{"fmt leaf + fmt-old wrap",
			&werrFmto{&errFmt{"woo"}, "waa"},
			waawoo, `
woo:
    -- verbose leaf:
    woo
-- verbose wrapper (fmt):
waa`, ``,
		},

		{"fmt leaf + fmt-partial wrap",
			&werrFmtp{&errFmt{"woo"}, "waa"},
			waawoo, `
waa:
  - woo:
    -- verbose leaf:
    woo`, ``,
		},

		{"fmt leaf + fmt wrap",
			&werrFmt{&errFmt{"woo"}, "waa"},
			waawoo, `
waa:
    -- verbose wrapper:
    waa
  - woo:
    -- verbose leaf:
    woo`, ``,
		},

		{"nofmt wrap in + nofmt wrap out",
			&werrNoFmt{&werrNoFmt{&errFmt{"woo"}, "waa"}, "wuu"},
			wuuwaawoo, wuuwaawoo, ``},

		{"nofmt wrap in + fmd-old wrap out",
			&werrFmto{&werrNoFmt{&errFmt{"woo"}, "waa"}, "wuu"},
			wuuwaawoo, `
waa: woo
-- verbose wrapper (fmt):
wuu`, ``,
		},

		{"nofmt wrap in + fmt wrap out",
			&werrFmt{&werrNoFmt{&errFmt{"woo"}, "waa"}, "wuu"},
			wuuwaawoo, `
wuu:
    -- verbose wrapper:
    wuu
  - waa
  - woo:
    -- verbose leaf:
    woo`, ``,
		},

		{"fmt-old wrap in + nofmt wrap out",
			&werrNoFmt{&werrFmto{&errFmt{"woo"}, "waa"}, "wuu"},
			wuuwaawoo, wuuwaawoo, ``},

		{"fmt-old wrap in + fmd-old wrap out",
			&werrFmto{&werrFmto{&errFmt{"woo"}, "waa"}, "wuu"},
			wuuwaawoo, `
woo:
    -- verbose leaf:
    woo
-- verbose wrapper (fmt):
waa
-- verbose wrapper (fmt):
wuu`, ``,
		},

		{"fmt-old wrap in + fmt wrap out",
			&werrFmt{&werrFmto{&errFmt{"woo"}, "waa"}, "wuu"},
			wuuwaawoo, `
wuu:
    -- verbose wrapper:
    wuu
  - woo:
        -- verbose leaf:
        woo
    -- verbose wrapper (fmt):
    waa`, ``,
		},

		{"fmt wrap in + nofmt wrap out",
			&werrNoFmt{&werrFmt{&errFmt{"woo"}, "waa"}, "wuu"},
			wuuwaawoo, wuuwaawoo, ``},

		{"fmt wrap in + fmd-old wrap out",
			&werrFmto{&werrFmt{&errFmt{"woo"}, "waa"}, "wuu"},
			wuuwaawoo, `
waa:
    -- verbose wrapper:
    waa
  - woo:
    -- verbose leaf:
    woo
-- verbose wrapper (fmt):
wuu`, ``,
		},

		{"fmt wrap in + fmt wrap out",
			&werrFmt{&werrFmt{&errFmt{"woo"}, "waa"}, "wuu"},
			wuuwaawoo, `
wuu:
    -- verbose wrapper:
    wuu
  - waa:
    -- verbose wrapper:
    waa
  - woo:
    -- verbose leaf:
    woo`, ``,
		},

		// Opaque leaf.
		{"opaque leaf",
			errbase.DecodeError(ctx, errbase.EncodeError(ctx, &errNoFmt{"woo"})),
			woo, `
woo:
    (opaque error leaf)
    type name: github.com/cockroachdb/errors/errbase_test/*errbase_test.errNoFmt`, ``},

		// Opaque wrapper.
		{"opaque wrapper",
			errbase.DecodeError(ctx, errbase.EncodeError(ctx, &werrNoFmt{goErr.New("woo"), "waa"})),
			waawoo, `
waa:
    (opaque error wrapper)
    type name: github.com/cockroachdb/errors/errbase_test/*errbase_test.werrNoFmt
  - woo`, ``},

		{"opaque wrapper+wrapper",
			errbase.DecodeError(ctx, errbase.EncodeError(ctx, &werrNoFmt{&werrNoFmt{goErr.New("woo"), "waa"}, "wuu"})),
			wuuwaawoo, `
wuu:
    (opaque error wrapper)
    type name: github.com/cockroachdb/errors/errbase_test/*errbase_test.werrNoFmt
  - waa:
    (opaque error wrapper)
    type name: github.com/cockroachdb/errors/errbase_test/*errbase_test.werrNoFmt
  - woo`, ``},

		// Compatibility with github.com/pkg/errors.

		{"pkg msg + fmt leaf",
			pkgErr.WithMessage(&errFmt{"woo"}, "waa"),
			waawoo, `
woo:
    -- verbose leaf:
    woo
waa`,
			// The implementation of (*pkgErr.withMessage).Format() is wrong for %q. Oh well...
			`waa: woo`,
		},

		{"fmt wrap + pkg msg + fmt leaf",
			&werrFmt{pkgErr.WithMessage(&errFmt{"woo"}, "waa"), "wuu"},
			wuuwaawoo, `
wuu:
    -- verbose wrapper:
    wuu
  - woo:
        -- verbose leaf:
        woo
    waa`, ``,
		},

		{"fmt wrap + pkg msg1 + pkg.msg2 + fmt leaf",
			&werrFmt{
				pkgErr.WithMessage(
					pkgErr.WithMessage(
						&errFmt{"woo"}, "waa2"),
					"waa1"),
				"wuu"},
			`wuu: waa1: waa2: woo`, `
wuu:
    -- verbose wrapper:
    wuu
  - woo:
        -- verbose leaf:
        woo
    waa2
    waa1`, ``,
		},

		{"fmt wrap + pkg stack + fmt leaf",
			&werrFmt{pkgErr.WithStack(&errFmt{"woo"}), "waa"},
			waawoo, `
waa:
    -- verbose wrapper:
    waa
  - woo:
        -- verbose leaf:
        woo
    github.com/cockroachdb/errors/errbase_test.TestFormat
    <tab><path>
    testing.tRunner
    <tab><path>
    runtime.goexit
    <tab><path>`, ``,
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
			ref := test.expFmtQuote
			if ref == "" {
				ref = fmt.Sprintf("%q", test.expFmtSimple)
			}
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
			fmt.Fprintf(s, "\n-- verbose leaf (fmt):\n%s", e.msg)
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
			fmt.Fprintf(s, "\n-- verbose wrapper (fmt):\n%s", e.msg)
			return
		}
		fallthrough
	case 's', 'q':
		fmt.Fprintf(s, fmt.Sprintf("%%%s%c", flags(s), verb), e.Error())
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
		p.Printf("-- verbose leaf:\n%s", e.msg)
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
		p.Printf("-- verbose wrapper:\n%s", e.msg)
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
