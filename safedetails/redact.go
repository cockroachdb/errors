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
	foundDetail := false
	if c := errbase.UnwrapOnce(err); c == nil {
		// This is a leaf error. Decode the leaf and return.
		foundDetail = redactLeafErr(buf, err)
	} else /* c != nil */ {
		// Print the inner error before the outer error.
		redactErr(buf, c)
		foundDetail = redactWrapper(buf, err)
	}

	// Add any additional safe strings from the wrapper, if present.
	if payload := errbase.GetSafeDetails(err); len(payload.SafeDetails) > 0 {
		consumed := 0
		if !foundDetail {
			firstDetail := strings.TrimSpace(payload.SafeDetails[0])
			if strings.IndexByte(firstDetail, '\n') < 0 {
				firstDetail = strings.ReplaceAll(strings.TrimSpace(payload.SafeDetails[0]), "\n", "\n  ")
				if len(firstDetail) > 0 {
					buf.WriteString(": ")
				}
				buf.WriteString(firstDetail)
				consumed = 1
				foundDetail = true
			}
		}
		if len(payload.SafeDetails) > consumed {
			buf.WriteString("\n  (more details:)")
			for _, sd := range payload.SafeDetails[consumed:] {
				buf.WriteString("\n  ")
				buf.WriteString(strings.ReplaceAll(strings.TrimSpace(sd), "\n", "\n  "))
			}
			foundDetail = true
		}
	}
	if !foundDetail {
		buf.WriteString(": <redacted>")
	}
}

func redactWrapper(buf *strings.Builder, err error) (hasDetail bool) {
	buf.WriteString("\n")
	switch t := err.(type) {
	case *os.SyscallError:
		hasDetail = true
		typAnd(buf, t, t.Syscall)
	case *os.PathError:
		hasDetail = true
		typAnd(buf, t, t.Op)
	case *os.LinkError:
		hasDetail = true
		fmt.Fprintf(buf, "%T: %s <redacted> <redacted>", t, t.Op)
	case *net.OpError:
		hasDetail = true
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
		fmt.Fprintf(buf, "%T", err)
	}
	return
}

func redactLeafErr(buf *strings.Builder, err error) (hasDetail bool) {
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
		hasDetail = true
		typAnd(buf, err, err.Error())
		return
	}

	if redactPre113Wrappers(buf, err) {
		hasDetail = true
		return
	}

	// The following two types are safe too.
	switch t := err.(type) {
	case runtime.Error:
		hasDetail = true
		typAnd(buf, t, t.Error())
	case syscall.Errno:
		hasDetail = true
		typAnd(buf, t, t.Error())
	case SafeMessager:
		hasDetail = true
		typAnd(buf, t, t.SafeMessage())
	default:
		// No further information about this error, simply report its type.
		fmt.Fprintf(buf, "%T", err)
	}
	return
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
