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

package hintdetail_test

import (
	"context"
	goErr "errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cockroachdb/errors/assert"
	"github.com/cockroachdb/errors/errbase"
	"github.com/cockroachdb/errors/hintdetail"
	"github.com/cockroachdb/errors/issuelink"
	"github.com/cockroachdb/errors/markers"
	"github.com/cockroachdb/errors/stdstrings"
	"github.com/cockroachdb/errors/testutils"
	"github.com/pkg/errors"
)

func TestDetail(t *testing.T) {
	tt := testutils.T{T: t}

	origErr := errors.New("world")

	err := errors.Wrap(
		hintdetail.WithDetail(
			errors.WithStack(
				hintdetail.WithDetail(origErr, "foo"),
			),
			"bar",
		),
		"hello")

	theTest := func(tt testutils.T, err error) {
		tt.Check(markers.Is(err, origErr))
		tt.CheckEqual(err.Error(), "hello: world")

		details := hintdetail.GetAllDetails(err)
		tt.CheckDeepEqual(details, []string{"foo", "bar"})

		errV := fmt.Sprintf("%+v", err)
		tt.Check(strings.Contains(errV, "detail: foo"))
		tt.Check(strings.Contains(errV, "detail: bar"))
	}

	tt.Run("local", func(tt testutils.T) { theTest(tt, err) })

	enc := errbase.EncodeError(context.Background(), err)
	newErr := errbase.DecodeError(context.Background(), enc)

	tt.Run("remote", func(tt testutils.T) { theTest(tt, newErr) })
}

func TestHint(t *testing.T) {
	tt := testutils.T{T: t}

	origErr := errors.New("world")

	err := errors.Wrap(
		hintdetail.WithHint(
			hintdetail.WithHint(
				errors.WithStack(
					hintdetail.WithHint(origErr, "foo"),
				),
				"bar",
			),
			"foo",
		),
		"hello")

	theTest := func(tt testutils.T, err error) {
		tt.Check(markers.Is(err, origErr))
		tt.CheckEqual(err.Error(), "hello: world")

		hints := hintdetail.GetAllHints(err)
		tt.CheckDeepEqual(hints, []string{"foo", "bar"})

		errV := fmt.Sprintf("%+v", err)
		tt.Check(strings.Contains(errV, "hint: foo"))
		tt.Check(strings.Contains(errV, "hint: bar"))
	}

	tt.Run("local", func(tt testutils.T) { theTest(tt, err) })

	enc := errbase.EncodeError(context.Background(), err)
	newErr := errbase.DecodeError(context.Background(), enc)

	tt.Run("remote", func(tt testutils.T) { theTest(tt, newErr) })
}

func TestIssueLinkHint(t *testing.T) {
	tt := testutils.T{T: t}

	err := issuelink.WithIssueLink(
		issuelink.WithIssueLink(
			errors.New("hello world"),
			issuelink.IssueLink{IssueURL: "foo"},
		),
		issuelink.IssueLink{IssueURL: "bar"},
	)

	theTest := func(tt testutils.T, err error) {
		tt.CheckEqual(err.Error(), "hello world")

		hints := hintdetail.GetAllHints(err)
		tt.Assert(len(hints) == 2)

		tt.CheckEqual(hints[0], "See: foo")
		tt.CheckEqual(hints[1], "See: bar")
	}

	tt.Run("local", func(tt testutils.T) { theTest(tt, err) })

	enc := errbase.EncodeError(context.Background(), err)
	newErr := errbase.DecodeError(context.Background(), enc)

	tt.Run("remote", func(tt testutils.T) { theTest(tt, newErr) })
}

func TestUnimplementedHint(t *testing.T) {
	tt := testutils.T{T: t}

	err := issuelink.UnimplementedError(issuelink.IssueLink{IssueURL: "woo"}, "hello world")

	theTest := func(tt testutils.T, err error) {
		tt.CheckEqual(err.Error(), "hello world")

		hints := hintdetail.GetAllHints(err)
		tt.Assert(len(hints) > 0)

		tt.CheckEqual(hints[0], issuelink.UnimplementedErrorHint+"\nSee: woo")
	}

	tt.Run("local", func(tt testutils.T) { theTest(tt, err) })

	enc := errbase.EncodeError(context.Background(), err)
	newErr := errbase.DecodeError(context.Background(), enc)

	tt.Run("remote", func(tt testutils.T) { theTest(tt, newErr) })
}

func TestUnimplementedNoIssueHint(t *testing.T) {
	tt := testutils.T{T: t}

	err := issuelink.UnimplementedError(issuelink.IssueLink{}, "hello world")

	theTest := func(tt testutils.T, err error) {
		tt.CheckEqual(err.Error(), "hello world")

		hints := hintdetail.GetAllHints(err)
		tt.Assert(len(hints) > 0)

		tt.CheckEqual(hints[0], issuelink.UnimplementedErrorHint+stdstrings.IssueReferral)
	}

	tt.Run("local", func(tt testutils.T) { theTest(tt, err) })

	enc := errbase.EncodeError(context.Background(), err)
	newErr := errbase.DecodeError(context.Background(), enc)

	tt.Run("remote", func(tt testutils.T) { theTest(tt, newErr) })
}

func TestAssertionHints(t *testing.T) {
	tt := testutils.T{T: t}

	err := assert.WithAssertionFailure(errors.New("hello world"))

	theTest := func(tt testutils.T, err error) {
		tt.CheckEqual(err.Error(), "hello world")

		hints := hintdetail.GetAllHints(err)
		tt.Assert(len(hints) > 0)

		tt.CheckEqual(hints[0], assert.AssertionErrorHint+stdstrings.IssueReferral)
	}

	tt.Run("local", func(tt testutils.T) { theTest(tt, err) })

	enc := errbase.EncodeError(context.Background(), err)
	newErr := errbase.DecodeError(context.Background(), enc)

	tt.Run("remote", func(tt testutils.T) { theTest(tt, newErr) })
}

func TestMultiHintDetail(t *testing.T) {
	tt := testutils.T{T: t}

	err := errors.New("hello world")
	err = hintdetail.WithHint(err, "woo")
	err = hintdetail.WithHint(err, "waa")

	tt.CheckEqual(hintdetail.FlattenHints(err), "woo\n--\nwaa")

	err = hintdetail.WithDetail(err, "foo")
	err = hintdetail.WithDetail(err, "bar")
	tt.CheckEqual(hintdetail.FlattenDetails(err), "foo\n--\nbar")
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
		{"hint",
			hintdetail.WithHint(baseErr, "a"),
			woo, `
error with user hint: a
  - woo`},
		{"detail",
			hintdetail.WithDetail(baseErr, "a"),
			woo, `
error with user detail: a
  - woo`},

		{"hint + wrapper",
			hintdetail.WithHint(&werrFmt{baseErr, "waa"}, "a"),
			waawoo, `
error with user hint: a
  - waa:
    -- verbose wrapper:
    waa
  - woo`},

		{"detail + wrapper",
			hintdetail.WithDetail(&werrFmt{baseErr, "waa"}, "a"),
			waawoo, `
error with user detail: a
  - waa:
    -- verbose wrapper:
    waa
  - woo`},

		{"wrapper + hint",
			&werrFmt{hintdetail.WithHint(baseErr, "a"), "waa"},
			waawoo, `
waa:
    -- verbose wrapper:
    waa
  - error with user hint: a
  - woo`},
		{"wrapper + detail",
			&werrFmt{hintdetail.WithDetail(baseErr, "a"), "waa"},
			waawoo, `
waa:
    -- verbose wrapper:
    waa
  - error with user detail: a
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
