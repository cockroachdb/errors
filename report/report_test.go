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

package report_test

import (
	goErr "errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cockroachdb/errors/domains"
	"github.com/cockroachdb/errors/errbase"
	"github.com/cockroachdb/errors/report"
	"github.com/cockroachdb/errors/safedetails"
	"github.com/cockroachdb/errors/testutils"
	"github.com/cockroachdb/errors/withstack"
	"github.com/cockroachdb/sentry-go"
	"github.com/kr/pretty"
)

// func TestReport2(t *testing.T) {
// 	client, err := sentry.NewClient(
// 		sentry.ClientOptions{
// 			Debug: true,
// 			Dsn:   "<URL HERE>",
// 		})
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	sentry.CurrentHub().BindClient(client)
//
// 	myErr := errutil.Newf("Hello %s %d", "world", redact.Safe(123))
// 	myErr = errutil.Wrapf(myErr, "some prefix %s", "unseen")
//
// 	if eventID := report.ReportError(myErr); eventID == "" {
// 		t.Fatal("eventID is empty")
// 	}
// 	sentry.Flush(2 * time.Second)
//
// }

func TestReport(t *testing.T) {
	var events []*sentry.Event

	client, err := sentry.NewClient(
		sentry.ClientOptions{
			// Install a Transport that locally records events rather than
			// sending them to Sentry over HTTP.
			Transport: interceptingTransport{
				SendFunc: func(event *sentry.Event) {
					events = append(events, event)
				},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	sentry.CurrentHub().BindClient(client)

	thisDomain := domains.NamedDomain("thisdomain")

	err = goErr.New("hello")
	err = safedetails.WithSafeDetails(err, "universe %d %s",
		safedetails.Safe(123), safedetails.Safe("multi\nline"))
	err = withstack.WithStack(err)
	err = domains.WithDomain(err, thisDomain)
	defer errbase.TestingWithEmptyMigrationRegistry()()

	err = wrapWithMigratedType(err)

	if eventID := report.ReportError(err); eventID == "" {
		t.Fatal("eventID is empty")
	}

	t.Logf("received events: %# v", pretty.Formatter(events))

	tt := testutils.T{T: t}

	tt.Assert(len(events) == 1)
	e := events[0]

	tt.Run("long message payload", func(tt testutils.T) {
		expectedLongMessage := `^\*errors.errorString
\*safedetails.withSafeDetails: universe 123 multi \(1\)
report_test.go:\d+: \*withstack.withStack \(top exception\)
\*domains\.withDomain: error domain: "thisdomain" \(2\)
\*report_test\.myWrapper
\(check the extra data payloads\)$`
		tt.CheckRegexpEqual(e.Message, expectedLongMessage)
	})

	tt.Run("valid extra details", func(tt testutils.T) {
		expectedTypes := `errors/*errors.errorString (*::)
github.com/cockroachdb/errors/safedetails/*safedetails.withSafeDetails (*::)
github.com/cockroachdb/errors/withstack/*withstack.withStack (*::)
github.com/cockroachdb/errors/domains/*domains.withDomain (*::error domain: "thisdomain")
github.com/cockroachdb/errors/report_test/*report_test.myWrapper (some/previous/path/prevpkg.prevType::)
`
		types := fmt.Sprintf("%s", e.Extra["error types"])
		tt.CheckEqual(types, expectedTypes)

		expectedDetail := `universe 123 multi
   line`
		detail := fmt.Sprintf("%s", e.Extra["1: details"])
		tt.CheckEqual(strings.TrimSpace(detail), expectedDetail)

		expectedDetail = string(thisDomain)
		detail = fmt.Sprintf("%s", e.Extra["2: details"])
		tt.CheckEqual(strings.TrimSpace(detail), expectedDetail)
	})

	hasStack := false

	for _, exc := range e.Exception {
		tt.Check(!hasStack)

		tt.Run("stack trace payload", func(tt testutils.T) {
			tt.CheckEqual(exc.Module, string(thisDomain))

			st := exc.Stacktrace
			if st == nil || len(st.Frames) < 1 {
				t.Error("stack trace too short")
			} else {
				f := st.Frames[len(st.Frames)-1]
				tt.Check(strings.HasSuffix(f.Filename, "report_test.go"))
				tt.Check(strings.HasSuffix(f.AbsPath, "report_test.go"))
				tt.Check(strings.HasSuffix(f.Module, "/report_test"))
				tt.CheckEqual(f.Function, "TestReport")
				tt.Check(f.Lineno != 0)
			}
		})
		hasStack = true
	}

	tt.Check(hasStack)
}

func wrapWithMigratedType(err error) error {
	errbase.RegisterTypeMigration("some/previous/path", "prevpkg.prevType", (*myWrapper)(nil))
	return &myWrapper{cause: err}
}

type myWrapper struct {
	cause error
}

func (w *myWrapper) Error() string { return w.cause.Error() }
func (w *myWrapper) Cause() error  { return w.cause }

// interceptingTransport is an implementation of sentry.Transport that
// delegates calls to the SendEvent method to the send function contained
// within.
type interceptingTransport struct {
	// SendFunc is the send callback.
	SendFunc func(event *sentry.Event)
}

var _ sentry.Transport = &interceptingTransport{}

// Flush implements the sentry.Transport interface.
func (it interceptingTransport) Flush(time.Duration) bool {
	return true
}

// Configure implements the sentry.Transport interface.
func (it interceptingTransport) Configure(sentry.ClientOptions) {
}

// SendEvent implements the sentry.Transport interface.
func (it interceptingTransport) SendEvent(event *sentry.Event) {
	it.SendFunc(event)
}
