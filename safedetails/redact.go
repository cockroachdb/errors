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

package safedetails

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"syscall"

	"github.com/cockroachdb/errors/errbase"
	"github.com/cockroachdb/errors/markers"
	"github.com/cockroachdb/errors/withstack"
)

// Redact returns a redacted version of the supplied item that is safe to use in
// anonymized reporting.
func Redact(r interface{}) string {
	var buf strings.Builder

	switch t := r.(type) {
	case SafeMessager:
		buf.WriteString(t.SafeMessage())
	case error:
		if file, line, _, ok := withstack.GetOneLineSource(t); ok {
			fmt.Fprintf(&buf, "%s:%d: ", file, line)
		}
		redactErr(&buf, t)
	default:
		typRedacted(&buf, r)
	}

	return buf.String()
}

func redactErr(buf *strings.Builder, err error) {
	if c := errbase.UnwrapOnce(err); c == nil {
		// This is a leaf error. Decode the leaf and return.
		redactLeafErr(buf, err)
	} else /* c != nil */ {
		// Print the inner error before the outer error.
		redactErr(buf, c)
		redactWrapper(buf, err)
	}

	// Add any additional safe strings from the wrapper, if present.
	if payload := errbase.GetSafeDetails(err); len(payload.SafeDetails) > 0 {
		buf.WriteString("\n(more details about this error:)")
		for _, sd := range payload.SafeDetails {
			buf.WriteByte('\n')
			buf.WriteString(strings.TrimSpace(sd))
		}
	}
}

func redactWrapper(buf *strings.Builder, err error) {
	buf.WriteString("\n")
	switch t := err.(type) {
	case *os.SyscallError:
		typAnd(buf, t, t.Syscall)
	case *os.PathError:
		typAnd(buf, t, t.Op)
	case *os.LinkError:
		fmt.Fprintf(buf, "%T: %s <redacted> <redacted>", t, t.Op)
	case *net.OpError:
		typAnd(buf, t, t.Op)
		if t.Net != "" {
			fmt.Fprintf(buf, " %s", t.Net)
		}
		if t.Source != nil {
			buf.WriteString(" <redacted>")
		}
		if t.Addr != nil {
			if t.Source != nil {
				buf.WriteString(" ->")
			}
			buf.WriteString(" <redacted>")
		}
	default:
		typRedacted(buf, err)
	}
}

func redactLeafErr(buf *strings.Builder, err error) {
	// Is it a sentinel error? These are safe.
	if markers.IsAny(err,
		context.DeadlineExceeded,
		context.Canceled,
		os.ErrInvalid,
		os.ErrPermission,
		os.ErrExist,
		os.ErrNotExist,
		os.ErrClosed,
		os.ErrNoDeadline,
	) {
		typAnd(buf, err, err.Error())
		return
	}

	if redactPre113Wrappers(buf, err) {
		return
	}

	// The following two types are safe too.
	switch t := err.(type) {
	case runtime.Error:
		typAnd(buf, t, t.Error())
	case syscall.Errno:
		typAnd(buf, t, t.Error())
	case SafeMessager:
		typAnd(buf, t, t.SafeMessage())
	default:
		// No further information about this error, simply report its type.
		typRedacted(buf, err)
	}
}

func typRedacted(buf *strings.Builder, r interface{}) {
	fmt.Fprintf(buf, "%T:<redacted>", r)
}

func typAnd(buf *strings.Builder, r interface{}, msg string) {
	if len(msg) == 0 {
		msg = "(empty string)"
	}
	fmt.Fprintf(buf, "%T: %s", r, msg)
}
