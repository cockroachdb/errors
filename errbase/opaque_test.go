package errbase

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/cockroachdb/errors/testutils"
	"github.com/kr/pretty"
)

func TestUnknownWrapperTraversalWithMessageOverride(t *testing.T) {
	// Simulating scenario where the new field on the opaque wrapper is dropped
	// in the middle of the chain by a node running an older version.

	origErr := fmt.Errorf("this is a wrapped err %w with a non-prefix wrap msg", errors.New("hello"))
	t.Logf("start err: %# v", pretty.Formatter(origErr))

	// Encode the error, this will use the encoder.
	enc := EncodeError(context.Background(), origErr)
	t.Logf("encoded: %# v", pretty.Formatter(enc))

	newErr := DecodeError(context.Background(), enc)
	t.Logf("decoded: %# v", pretty.Formatter(newErr))

	// simulate node not knowing about `WrapperOwnsErrorString` proto field
	newErr.(*opaqueWrapper).ownsErrorString = false

	// Encode it again, to simulate the error passed on to another system.
	enc2 := EncodeError(context.Background(), newErr)
	t.Logf("encoded2: %# v", pretty.Formatter(enc))

	// Then decode again.
	newErr2 := DecodeError(context.Background(), enc2)
	t.Logf("decoded: %# v", pretty.Formatter(newErr2))

	tt := testutils.T{T: t}

	// We expect to see an erroneous `: hello` because our
	// error passes through a node which drops the new protobuf
	// field.
	tt.CheckEqual(newErr2.Error(), origErr.Error()+": hello")
}
