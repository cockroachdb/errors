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

package secondary

import "github.com/cockroachdb/errors/errbase"

// WithSecondaryError enhances the error given as first argument with
// an annotation that carries the error given as second argument.  The
// second error does not participate in cause analysis (Is, etc) and
// is only revealed when printing out the error or collecting safe
// (PII-free) details for reporting.
//
// If additionalErr is nil, the first error is returned as-is.
//
// Tip: consider using CombineErrors() below in the general case.
//
// Detail is shown:
// - via `errors.GetSafeDetails()`, shows details from secondary error.
// - when formatting with `%+v`.
// - in Sentry reports.
func WithSecondaryError(err error, additionalErr error) error {
	if err == nil || additionalErr == nil {
		return err
	}
	return &withSecondaryError{cause: err, secondaryError: additionalErr}
}

// CombineErrors returns err, or, if err is nil, otherErr.
// if err is non-nil, otherErr is attached as secondary error.
// See the documentation of `WithSecondaryError()` for details.
func CombineErrors(err error, otherErr error) error {
	if err == nil {
		return otherErr
	}
	return WithSecondaryError(err, otherErr)
}

// SummarizeErrors reduces a collection of errors to a single
// error with the rest as secondary errors, making an effort
// at deduplication. Use when it's not clear, or not deterministic,
// which of many errors will be the root cause.
func SummarizeErrors(errs ...error) error {
	if len(errs) == 0 {
		return nil
	}
	uniqArgsInOrder := make([]error, 0, len(errs))
	uniqArgsMap := make(map[error]struct{}, len(errs))
	refCount := make(map[error]int)
	for _, e := range errs {
		if _, dup := uniqArgsMap[e]; !dup {
			uniqArgsMap[e] = struct{}{}
			uniqArgsInOrder = append(uniqArgsInOrder, e)
			walk(e, func(w error) { refCount[w] = refCount[w] + 1 })
		}
	}
	var retVal error
	for _, e := range uniqArgsInOrder {
		if refCount[e] == 1 {
			retVal = CombineErrors(retVal, e)
		}
	}
	return retVal
}

func walk(err error, fn func(error)) {
	if err != nil {
		fn(err)
		walk(errbase.UnwrapOnce(err), fn)
		if se, ok := err.(*withSecondaryError); ok {
			walk(se.secondaryError, fn)
		}
	}
}
