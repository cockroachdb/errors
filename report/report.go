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

package report

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/cockroachdb/errors/domains"
	"github.com/cockroachdb/errors/errbase"
	"github.com/cockroachdb/errors/withstack"
	"github.com/cockroachdb/sentry-go"
)

// BuildSentryReport builds the components of a sentry report.  This
// can be used instead of ReportError() below to use additional custom
// conditions in the reporting or add additional reporting tags.
func BuildSentryReport(err error) (event *sentry.Event, extraDetails map[string]interface{}) {
	if err == nil {
		// No error: do nothing.
		return
	}

	var stacks []*withstack.ReportableStackTrace
	var details []errbase.SafeDetailPayload
	// Peel the error.
	for c := err; c != nil; c = errbase.UnwrapOnce(c) {
		st := withstack.GetReportableStackTrace(c)
		stacks = append(stacks, st)

		sd := errbase.GetSafeDetails(c)
		details = append(details, sd)
	}

	// A report can contain at most one "message", any number of
	// "exceptions", and arbitrarily many "extra" fields.
	//
	// So we populate the event as follow:
	// - the "message" will contain the type of the first error
	// - the "exceptions" will contain the details with
	//   populated encoded exceptions field.
	// - the "extra" will contain all the encoded stack traces
	//   or safe detail arrays.

	var firstError *string
	var exceptions []*withstack.ReportableStackTrace
	extras := make(map[string]interface{})
	var longMsgBuf bytes.Buffer
	var typesBuf bytes.Buffer

	extraNum := 1
	sep := ""
	for i := len(details) - 1; i >= 0; i-- {
		longMsgBuf.WriteString(sep)
		sep = "\n"

		// Collect the type name.
		tn := details[i].OriginalTypeName
		mark := details[i].ErrorTypeMark
		fm := "*"
		if tn != mark.FamilyName {
			fm = mark.FamilyName
		}
		fmt.Fprintf(&typesBuf, "%s (%s::%s)\n", tn, fm, mark.Extension)

		// Compose the message for this layer. The message consists of:
		// - optionally, a file/line reference, if a stack trace was available.
		// - the error/wrapper type name, with file prefix removed.
		// - optionally, the first line of the first detail string, if one is available.
		// - optionally, references to stack trace / details.
		if stacks[i] != nil && len(stacks[i].Frames) > 0 {
			f := stacks[i].Frames[len(stacks[i].Frames)-1]
			fn := f.Filename
			if j := strings.LastIndexByte(fn, '/'); j >= 0 {
				fn = fn[j+1:]
			}
			fmt.Fprintf(&longMsgBuf, "%s:%d: ", fn, f.Lineno)
		}

		longMsgBuf.WriteString(simpleErrType(tn))

		var genExtra bool

		// Is there a stack trace?
		if st := stacks[i]; st != nil {
			// Yes: generate the extra and list it on the line.
			stKey := fmt.Sprintf("%d: stacktrace", extraNum)
			extras[stKey] = PrintStackTrace(st)
			fmt.Fprintf(&longMsgBuf, " (%d)", extraNum)
			extraNum++

			exceptions = append(exceptions, st)
		} else {
			// No: are there details? If so, print them.
			// Note: we only print the details if no stack trace
			// was found that that level. This is because
			// stack trace annotations also produce the stack
			// trace as safe detail string.
			genExtra = len(details[i].SafeDetails) > 1
			if len(details[i].SafeDetails) > 0 {
				d := details[i].SafeDetails[0]
				if d != "" {
					genExtra = true
				}
				if j := strings.IndexByte(d, '\n'); j >= 0 {
					d = d[:j]
				}
				if d != "" {
					longMsgBuf.WriteString(": ")
					longMsgBuf.WriteString(d)
					if firstError == nil {
						// Keep the string for later.
						firstError = &d
					}
				}
			}
		}

		// Are we generating another extra for the safe detail strings?
		if genExtra {
			stKey := fmt.Sprintf("%d: details", extraNum)
			var extraStr bytes.Buffer
			for _, d := range details[i].SafeDetails {
				fmt.Fprintln(&extraStr, d)
			}
			extras[stKey] = extraStr.String()
			fmt.Fprintf(&longMsgBuf, " (%d)", extraNum)
			extraNum++
		}
	}

	// Determine a head message for the report.
	headMsg := "<unknown error>"
	if firstError != nil {
		headMsg = *firstError
	}
	// Prepend the "main" source line information if available/found.
	if file, line, fn, ok := withstack.GetOneLineSource(err); ok {
		headMsg = fmt.Sprintf("%s:%d: %s: %s", file, line, fn, headMsg)
	}

	extras["error types"] = typesBuf.String()

	// Make the message part more informational.
	longMsgBuf.WriteString("\n(check the extra data payloads)")
	extras["long message"] = longMsgBuf.String()

	event = sentry.NewEvent()
	event.Message = headMsg

	module := domains.GetDomain(err)
	for _, exception := range exceptions {
		event.Exception = append(event.Exception,
			sentry.Exception{
				Type:       "<reported error>",
				Module:     string(module),
				Stacktrace: exception,
			})
	}

	return event, extras
}

// ReportError reports the given error to Sentry. The caller is responsible for
// checking whether telemetry is enabled.
// Note: an empty 'eventID' can be returned which signifies that the error was
// not reported. This can occur when Sentry client hasn't been properly
// configured or Sentry client decided to not report the error (due to
// configured sampling rate, callbacks, Sentry's event processors, etc).
func ReportError(err error) (eventID string) {
	event, extraDetails := BuildSentryReport(err)

	for extraKey, extraValue := range extraDetails {
		event.Extra[extraKey] = extraValue
	}

	// Avoid leaking the machine's hostname by injecting the literal "<redacted>".
	// Otherwise, sentry.Client.Capture will see an empty ServerName field and
	// automatically fill in the machine's hostname.
	event.ServerName = "<redacted>"

	tags := map[string]string{
		"report_type": "error",
	}
	for key, value := range tags {
		event.Tags[key] = tags[value]
	}

	res := sentry.CaptureEvent(event)
	if res != nil {
		eventID = string(*res)
	}
	return
}

func simpleErrType(tn string) string {
	// Strip the path prefix.
	if i := strings.LastIndexByte(tn, '/'); i >= 0 {
		tn = tn[i+1:]
	}
	return tn
}
