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

package errbase

import (
	goErr "errors"
	"fmt"

	"github.com/cockroachdb/errors/errorspb"
	"github.com/cockroachdb/redact"
)

// opaqueLeaf is used when receiving an unknown leaf type.
// Its important property is that if it is communicated
// back to some network system that _does_ know about
// the type, the original object can be restored.
type opaqueLeaf struct {
	msg     string
	details errorspb.EncodedErrorDetails
}

var _ error = (*opaqueLeaf)(nil)
var _ SafeDetailer = (*opaqueLeaf)(nil)
var _ fmt.Formatter = (*opaqueLeaf)(nil)
var _ SafeFormatter = (*opaqueLeaf)(nil)

// opaqueWrapper is used when receiving an unknown wrapper type.
// Its important property is that if it is communicated
// back to some network system that _does_ know about
// the type, the original object can be restored.
type opaqueWrapper struct {
	cause   error
	prefix  string
	details errorspb.EncodedErrorDetails
}

type opaqueMultiWrapper struct {
	causes  []error
	prefix  string
	details errorspb.EncodedErrorDetails
}

var _ error = (*opaqueWrapper)(nil)
var _ SafeDetailer = (*opaqueWrapper)(nil)
var _ fmt.Formatter = (*opaqueWrapper)(nil)
var _ SafeFormatter = (*opaqueWrapper)(nil)

var _ error = (*opaqueMultiWrapper)(nil)
var _ SafeDetailer = (*opaqueMultiWrapper)(nil)
var _ fmt.Formatter = (*opaqueMultiWrapper)(nil)
var _ SafeFormatter = (*opaqueMultiWrapper)(nil)

func (e *opaqueLeaf) Error() string { return e.msg }

func (e *opaqueWrapper) Error() string {
	if e.prefix == "" {
		return e.cause.Error()
	}
	// TODO(davidh): cleaner way to do this?
	if e.details.ErrorTypeMark.FamilyName == "fmt/*fmt.wrapError" {
		return e.prefix
	}
	return fmt.Sprintf("%s: %s", e.prefix, e.cause)
}

// TODO(davidh): probably shouldn't create a join obj here
func (e *opaqueMultiWrapper) Error() string {
	if e.prefix == "" {
		return goErr.Join(e.causes...).Error()
	}
	return e.prefix
}

// the opaque wrapper is a wrapper.
func (e *opaqueWrapper) Cause() error  { return e.cause }
func (e *opaqueWrapper) Unwrap() error { return e.cause }

func (e *opaqueMultiWrapper) Unwrap() []error { return e.causes }

func (e *opaqueLeaf) SafeDetails() []string         { return e.details.ReportablePayload }
func (e *opaqueWrapper) SafeDetails() []string      { return e.details.ReportablePayload }
func (e *opaqueMultiWrapper) SafeDetails() []string { return e.details.ReportablePayload }

func (e *opaqueLeaf) Format(s fmt.State, verb rune)    { FormatError(e, s, verb) }
func (e *opaqueWrapper) Format(s fmt.State, verb rune) { FormatError(e, s, verb) }

// TODO(davidh): this probably needs work in FormatError
func (e *opaqueMultiWrapper) Format(s fmt.State, verb rune) { FormatError(e, s, verb) }

func (e *opaqueLeaf) SafeFormatError(p Printer) (next error) {
	p.Print(e.msg)
	if p.Detail() {
		p.Printf("\n(opaque error leaf)")
		p.Printf("\ntype name: %s", redact.Safe(e.details.OriginalTypeName))
		for i, d := range e.details.ReportablePayload {
			p.Printf("\nreportable %d:\n%s", redact.Safe(i), redact.Safe(d))
		}
		if e.details.FullDetails != nil {
			p.Printf("\npayload type: %s", redact.Safe(e.details.FullDetails.TypeUrl))
		}
	}
	return nil
}

func (e *opaqueWrapper) SafeFormatError(p Printer) (next error) {
	if len(e.prefix) > 0 {
		// We use the condition if len(msg) > 0 because
		// otherwise an empty string would cause a "redactable
		// empty string" to be emitted (something that looks like "<>")
		// and the error formatting code only cleanly elides
		// the prefix properly if the output string is completely empty.
		p.Print(e.prefix)
	}
	if p.Detail() {
		p.Printf("\n(opaque error wrapper)")
		p.Printf("\ntype name: %s", redact.Safe(e.details.OriginalTypeName))
		for i, d := range e.details.ReportablePayload {
			p.Printf("\nreportable %d:\n%s", redact.Safe(i), redact.Safe(d))
		}
		if e.details.FullDetails != nil {
			p.Printf("\npayload type: %s", redact.Safe(e.details.FullDetails.TypeUrl))
		}
	}
	return e.cause
}

func (e *opaqueMultiWrapper) SafeFormatError(p Printer) (next error) {
	if len(e.prefix) > 0 {
		// We use the condition if len(msg) > 0 because
		// otherwise an empty string would cause a "redactable
		// empty string" to be emitted (something that looks like "<>")
		// and the error formatting code only cleanly elides
		// the prefix properly if the output string is completely empty.
		p.Print(e.prefix)
	}
	if p.Detail() {
		p.Printf("\n(opaque error wrapper)")
		p.Printf("\ntype name: %s", redact.Safe(e.details.OriginalTypeName))
		for i, d := range e.details.ReportablePayload {
			p.Printf("\nreportable %d:\n%s", redact.Safe(i), redact.Safe(d))
		}
		if e.details.FullDetails != nil {
			p.Printf("\npayload type: %s", redact.Safe(e.details.FullDetails.TypeUrl))
		}
	}
	// TODO(davidh): ughhh what happens hear? how does the causal chain work in this case???????
	return nil
}
