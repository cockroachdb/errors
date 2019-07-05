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

package telemetrykeys_test

import (
	"context"
	goErr "errors"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/cockroachdb/errors/errbase"
	"github.com/cockroachdb/errors/markers"
	"github.com/cockroachdb/errors/telemetrykeys"
	"github.com/cockroachdb/errors/testutils"
	"github.com/pkg/errors"
)

func TestTelemetry(t *testing.T) {
	tt := testutils.T{T: t}

	baseErr := errors.New("world")
	err := errors.Wrap(
		telemetrykeys.WithTelemetry(
			telemetrykeys.WithTelemetry(
				baseErr,
				"a", "b"),
			"b", "c"),
		"hello")

	tt.Check(markers.Is(err, baseErr))
	tt.CheckEqual(err.Error(), "hello: world")

	keys := telemetrykeys.GetTelemetryKeys(err)
	sort.Strings(keys)
	tt.CheckDeepEqual(keys, []string{"a", "b", "c"})

	errV := fmt.Sprintf("%+v", err)
	tt.Check(strings.Contains(errV, `keys: [a b]`))
	tt.Check(strings.Contains(errV, `keys: [b c]`))

	enc := errbase.EncodeError(context.Background(), err)
	newErr := errbase.DecodeError(context.Background(), enc)

	tt.Check(markers.Is(newErr, baseErr))
	tt.CheckEqual(newErr.Error(), "hello: world")

	keys = telemetrykeys.GetTelemetryKeys(newErr)
	sort.Strings(keys)
	tt.CheckDeepEqual(keys, []string{"a", "b", "c"})

	errV = fmt.Sprintf("%+v", newErr)
	tt.Check(strings.Contains(errV, `keys: [a b]`))
	tt.Check(strings.Contains(errV, `keys: [b c]`))
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
		{"keys",
			telemetrykeys.WithTelemetry(baseErr, "a", "b"),
			woo, `
error with telemetry keys: [a b]
  - woo`},

		{"keys + wrapper",
			telemetrykeys.WithTelemetry(&werrFmt{baseErr, "waa"}, "a", "b"),
			waawoo, `
error with telemetry keys: [a b]
  - waa:
    -- verbose wrapper:
    waa
  - woo`},

		{"wrapper + keys",
			&werrFmt{telemetrykeys.WithTelemetry(baseErr, "a", "b"), "waa"},
			waawoo, `
waa:
    -- verbose wrapper:
    waa
  - error with telemetry keys: [a b]
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
