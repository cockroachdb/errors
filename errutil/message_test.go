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

	const wuuwaawoo = `wuu: waa: woo`
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
			wuuwaawoo, `
wuu:
    -- verbose wrapper:
    wuu
  - waa:
  - woo`,
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
