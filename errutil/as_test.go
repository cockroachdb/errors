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

package errutil_test

import (
	"fmt"
	"testing"

	"github.com/interspace/errors"
	"github.com/interspace/errors/testutils"
)

func TestAs(t *testing.T) {
	tt := testutils.T{t}

	refErr := &myType{msg: "woo"}

	// Check we can fish the leaf back.
	var mySlot *myType
	tt.Check(errors.As(refErr, &mySlot))
	tt.Check(errors.Is(mySlot, refErr))

	// Check we can fish it even if behind something else.
	// Note: this would fail with xerrors.As() because
	// Wrap() uses github.com/pkg/errors which implements
	// Cause() but not Unwrap().
	// This may change with https://github.com/pkg/errors/pull/206.
	wErr := errors.Wrap(refErr, "hidden")
	mySlot = nil
	tt.Check(errors.As(wErr, &mySlot))
	tt.Check(errors.Is(mySlot, refErr))

	// Check we can fish the wrapper back.
	refwErr := &myWrapper{cause: errors.New("world"), msg: "hello"}
	var mywSlot *myWrapper
	tt.Check(errors.As(refwErr, &mywSlot))
	tt.Check(errors.Is(mywSlot, refwErr))

	// Check that it works even if behind something else.
	wwErr := errors.Wrap(refwErr, "hidden")
	mywSlot = nil
	tt.Check(errors.As(wwErr, &mywSlot))
	tt.Check(errors.Is(mywSlot, refwErr))
}

type myType struct{ msg string }

func (m *myType) Error() string { return m.msg }

type myWrapper struct {
	cause error
	msg   string
}

func (m *myWrapper) Error() string { return fmt.Sprintf("%s: %v", m.msg, m.cause) }
