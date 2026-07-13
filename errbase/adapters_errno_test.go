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

//go:build !plan9
// +build !plan9

package errbase_test

import (
	"context"
	"reflect"
	"syscall"
	"testing"

	"github.com/cockroachdb/errors/errbase"
	"github.com/cockroachdb/errors/errorspb"
	"github.com/cockroachdb/errors/oserror"
	"github.com/cockroachdb/errors/testutils"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestAdaptErrno(t *testing.T) {
	tt := testutils.T{T: t}

	// Arbitrary values of errno on a given platform are preserved
	// exactly when decoded on the same platform.
	origErr := syscall.Errno(123)
	newErr := network(t, origErr)
	tt.Check(reflect.DeepEqual(newErr, origErr))

	// Common values of errno preserve their properties
	// across a network encode/decode even though they
	// may not decode to the same type.
	for i := 0; i < 2000; i++ {
		origErr := syscall.Errno(i)
		enc := errbase.EncodeError(context.Background(), origErr)

		// Trick the decoder into thinking the error comes from a different platform.
		details := enc.Error.(*errorspb.EncodedError_Leaf).Leaf.Details
		d, err := details.FullDetails.UnmarshalNew()
		if err != nil {
			t.Fatal(err)
		}
		errnoDetails := d.(*errorspb.ErrnoPayload)
		errnoDetails.Arch = "OTHER"
		any, err := anypb.New(errnoDetails)
		if err != nil {
			t.Fatal(err)
		}
		details.FullDetails = any

		// Now decode the error. This produces an OpaqueErrno payload.
		dec := errbase.DecodeError(context.Background(), &enc)
		if _, ok := dec.(*errbase.OpaqueErrno); !ok {
			t.Fatalf("expected OpaqueErrno, got %T", dec)
		}

		// Now check that the properties have been preserved properly.
		tt.CheckEqual(oserror.IsPermission(origErr), oserror.IsPermission(dec))
		tt.CheckEqual(oserror.IsExist(origErr), oserror.IsExist(dec))
		tt.CheckEqual(oserror.IsNotExist(origErr), oserror.IsNotExist(dec))
		tt.CheckEqual(oserror.IsTimeout(origErr), oserror.IsTimeout(dec))
	}
}
