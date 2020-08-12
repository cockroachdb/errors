// Copyright 2020 The Cockroach Authors.
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

package safedetails_test

import (
	"context"
	"errors"
	"net"
	"os"
	"regexp"
	"runtime"
	"testing"

	"github.com/cockroachdb/errors/safedetails"
	"github.com/cockroachdb/errors/testutils"
	"github.com/cockroachdb/errors/withstack"
)

func TestRedact(t *testing.T) {
	errSentinel := (error)(struct{ error }{})

	testData := []struct {
		obj      interface{}
		expected string
	}{
		// Redacting non-error values.

		{123, `<int>`},
		{"secret", `<string>`},

		// Redacting SafeMessagers.

		{mySafer{}, `hello`},
		{safedetails.Safe(123), `123`},
		{mySafeError{}, `hello`},
		{&werrFmt{mySafeError{}, "unseen"},
			`safedetails_test.mySafeError: hello
wrapper: <*safedetails_test.werrFmt>`},

		// Redacting errors.

		// Unspecial cases, get redacted.
		{errors.New("secret"), `<*errors.errorString>`},

		// Stack trace in error retrieves some info about the context.
		{withstack.WithStack(errors.New("secret")),
			`<path>: <*errors.errorString>
wrapper: <*withstack.withStack>
(more details:)
github.com/cockroachdb/errors/safedetails_test.TestRedact
	<path>
testing.tRunner
	<path>
runtime.goexit
	<path>`},

		// Special cases, unredacted.
		{os.ErrInvalid, `*errors.errorString: invalid argument`},
		{os.ErrPermission, `*errors.errorString: permission denied`},
		{os.ErrExist, `*errors.errorString: file already exists`},
		{os.ErrNotExist, `*errors.errorString: file does not exist`},
		{os.ErrClosed, `*errors.errorString: file already closed`},
		{os.ErrNoDeadline, `*errors.errorString: file type does not support deadline`},

		{context.Canceled,
			`*errors.errorString: context canceled`},
		{context.DeadlineExceeded,
			`context.deadlineExceededError: context deadline exceeded`},

		{makeTypeAssertionErr(),
			`*runtime.TypeAssertionError: interface conversion: interface {} is nil, not int`},

		{errSentinel, // explodes if Error() called
			`<struct { error }>`},

		{&werrFmt{&werrFmt{os.ErrClosed, "unseen"}, "unsung"},
			`*errors.errorString: file already closed
wrapper: <*safedetails_test.werrFmt>
wrapper: <*safedetails_test.werrFmt>`},

		// Special cases, get partly redacted.

		{os.NewSyscallError("rename", os.ErrNotExist),
			`*errors.errorString: file does not exist
wrapper: *os.SyscallError: rename`},

		{&os.PathError{Op: "rename", Path: "secret", Err: os.ErrNotExist},
			`*errors.errorString: file does not exist
wrapper: *os.PathError: rename`},

		{&os.LinkError{
			Op:  "moo",
			Old: "sec",
			New: "cret",
			Err: os.ErrNotExist,
		},
			`*errors.errorString: file does not exist
wrapper: *os.LinkError: moo <redacted> <redacted>`},

		{&net.OpError{
			Op:     "write",
			Net:    "tcp",
			Source: &net.IPAddr{IP: net.IP("sensitive-source")},
			Addr:   &net.IPAddr{IP: net.IP("sensitive-addr")},
			Err:    errors.New("not safe"),
		}, `<*errors.errorString>
wrapper: *net.OpError: write tcp<redacted>-><redacted>`},
	}

	tt := testutils.T{T: t}

	for _, tc := range testData {
		s := safedetails.Redact(tc.obj)
		s = fileref.ReplaceAllString(s, "<path>")

		tt.CheckStringEqual(s, tc.expected)
	}
}

var fileref = regexp.MustCompile(`([a-zA-Z0-9\._/@-]*\.(?:go|s):\d+)`)

// makeTypeAssertionErr returns a runtime.Error with the message:
//     interface conversion: interface {} is nil, not int
func makeTypeAssertionErr() (result runtime.Error) {
	defer func() {
		e := recover()
		result = e.(runtime.Error)
	}()
	var x interface{}
	_ = x.(int)
	return nil
}

type mySafer struct{}

func (mySafer) SafeMessage() string { return "hello" }

type mySafeError struct{}

func (mySafeError) SafeMessage() string { return "hello" }
func (mySafeError) Error() string       { return "helloerr" }
