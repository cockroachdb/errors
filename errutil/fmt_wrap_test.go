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

// +build go1.13

package errutil_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/cockroachdb/errors/errutil"
	"github.com/cockroachdb/errors/testutils"
)

func TestErrorWrap(t *testing.T) {
	tt := testutils.T{t}

	baseErr := errutil.New("world")

	testCases := []struct {
		name          string
		err           error
		expFmtSimple  string
		expFmtVerbose string
	}{
		{"fmt wrap err",
			fmt.Errorf("hello: %w", baseErr),
			`hello: world`,
			// This fails to reveal the errors details because
			// fmt.Error's error objects do not implement a full %+v.
			`hello: world`},

		{"fmt rewrap err",
			errutil.Wrap(fmt.Errorf("hello: %w", baseErr), "woo"),
			`woo: hello: world`,
			// However, ours do.
			`error with attached stack trace:
    github.com/cockroachdb/errors/errutil_test.TestErrorWrap
    <tab><path>
    testing.tRunner
    <tab><path>
    runtime.goexit
    <tab><path>
  - woo:
  - hello
  - error with attached stack trace:
    github.com/cockroachdb/errors/errutil_test.TestErrorWrap
    <tab><path>
    testing.tRunner
    <tab><path>
    runtime.goexit
    <tab><path>
  - world`},

		{"new wrap err",
			errutil.Newf("hello: %w", baseErr),
			`hello: world`,
			`error with attached stack trace:
    github.com/cockroachdb/errors/errutil_test.TestErrorWrap
    <tab><path>
    testing.tRunner
    <tab><path>
    runtime.goexit
    <tab><path>
  - error with embedded safe details: hello: %w
    -- arg 1: <*errors.errorString>
    wrapper: <*withstack.withStack>
    (more details:)
    github.com/cockroachdb/errors/errutil_test.TestErrorWrap
    <tab><path>
    testing.tRunner
    <tab><path>
    runtime.goexit
    <tab><path>
  - hello
  - error with attached stack trace:
    github.com/cockroachdb/errors/errutil_test.TestErrorWrap
    <tab><path>
    testing.tRunner
    <tab><path>
    runtime.goexit
    <tab><path>
  - world`},
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
