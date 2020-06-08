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

	"github.com/interspace/errors/errutil"
	"github.com/interspace/errors/testutils"
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
			`woo: hello: world
(1) attached stack trace
  | github.com/interspace/errors/errutil_test.TestErrorWrap
  | <tab><path>
  | [...repeated from below...]
Wraps: (2) woo
Wraps: (3) hello
Wraps: (4) attached stack trace
  | github.com/interspace/errors/errutil_test.TestErrorWrap
  | <tab><path>
  | testing.tRunner
  | <tab><path>
  | runtime.goexit
  | <tab><path>
Wraps: (5) world
Error types: (1) *withstack.withStack (2) *errutil.withMessage (3) *fmt.wrapError (4) *withstack.withStack (5) *errors.errorString`},

		{"new wrap err",
			errutil.Newf("hello: %w", baseErr),
			`hello: world`,
			`hello: world
(1) attached stack trace
  | github.com/interspace/errors/errutil_test.TestErrorWrap
  | <tab><path>
  | [...repeated from below...]
Wraps: (2) 2 safe details enclosed
Wraps: (3) hello
Wraps: (4) attached stack trace
  | github.com/interspace/errors/errutil_test.TestErrorWrap
  | <tab><path>
  | testing.tRunner
  | <tab><path>
  | runtime.goexit
  | <tab><path>
Wraps: (5) world
Error types: (1) *withstack.withStack (2) *safedetails.withSafeDetails (3) *fmt.wrapError (4) *withstack.withStack (5) *errors.errorString`},
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
