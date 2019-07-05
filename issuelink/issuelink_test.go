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

package issuelink_test

import (
	"context"
	goErr "errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cockroachdb/errors/errbase"
	"github.com/cockroachdb/errors/issuelink"
	"github.com/cockroachdb/errors/markers"
	"github.com/cockroachdb/errors/testutils"
	"github.com/pkg/errors"
)

func TestIssueLink(t *testing.T) {
	tt := testutils.T{T: t}

	origErr := errors.New("world")

	err := errors.Wrap(
		issuelink.WithIssueLink(
			errors.WithStack(
				issuelink.WithIssueLink(origErr, issuelink.IssueLink{IssueURL: "123", Detail: "foo"}),
			),
			issuelink.IssueLink{IssueURL: "456", Detail: "bar"},
		),
		"hello")

	theTest := func(tt testutils.T, err error) {
		tt.Check(markers.Is(err, origErr))
		tt.CheckEqual(err.Error(), "hello: world")

		tt.Check(issuelink.HasIssueLink(err))
		if _, ok := markers.If(err, func(err error) (interface{}, bool) { return nil, issuelink.IsIssueLink(err) }); !ok {
			t.Error("woops")
		}

		details := issuelink.GetAllIssueLinks(err)
		tt.CheckDeepEqual(details, []issuelink.IssueLink{
			{IssueURL: "456", Detail: "bar"},
			{IssueURL: "123", Detail: "foo"},
		})

		errV := fmt.Sprintf("%+v", err)
		tt.Check(strings.Contains(errV, "issue: 123"))
		tt.Check(strings.Contains(errV, "issue: 456"))
		tt.Check(strings.Contains(errV, "detail: foo"))
		tt.Check(strings.Contains(errV, "detail: bar"))
	}

	tt.Run("local", func(tt testutils.T) { theTest(tt, err) })

	enc := errbase.EncodeError(context.Background(), err)
	newErr := errbase.DecodeError(context.Background(), enc)

	tt.Run("remote", func(tt testutils.T) { theTest(tt, newErr) })

}

func TestFormat(t *testing.T) {
	tt := testutils.T{t}

	baseErr := goErr.New("woo")
	link := issuelink.IssueLink{IssueURL: "http://mysite"}
	const woo = `woo`
	const waawoo = `waa: woo`
	testCases := []struct {
		name          string
		err           error
		expFmtSimple  string
		expFmtVerbose string
	}{
		{"link",
			issuelink.WithIssueLink(baseErr, link),
			woo, `
error with linked issue
    issue: http://mysite
  - woo`},
		{"link-details",
			issuelink.WithIssueLink(baseErr, issuelink.IssueLink{IssueURL: "http://mysite", Detail: "see more"}),
			woo, `
error with linked issue
    issue: http://mysite
    detail: see more
  - woo`},

		{"link + wrapper",
			issuelink.WithIssueLink(&werrFmt{baseErr, "waa"}, link),
			waawoo, `
error with linked issue
    issue: http://mysite
  - waa:
    -- verbose wrapper:
    waa
  - woo`},

		{"wrapper + link",
			&werrFmt{issuelink.WithIssueLink(baseErr, link), "waa"},
			waawoo, `
waa:
    -- verbose wrapper:
    waa
  - error with linked issue
    issue: http://mysite
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
