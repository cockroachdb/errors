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

package contexttags_test

import (
	"context"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/errors/contexttags"
	"github.com/cockroachdb/errors/errbase"
	"github.com/cockroachdb/errors/markers"
	"github.com/cockroachdb/errors/testutils"
	"github.com/cockroachdb/logtags"
)

func TestWithContext(t *testing.T) {
	tt := testutils.T{T: t}

	// Create an example context with decoration.
	ctx := context.Background()
	ctx = logtags.AddTag(ctx, "foo", 123)
	ctx = logtags.AddTag(ctx, "x", 456)
	ctx = logtags.AddTag(ctx, "bar", nil)

	// Create another example context. We use two to demonstrate that an
	// error can store multiple sets of context tags.
	ctx2 := context.Background()
	ctx2 = logtags.AddTag(ctx2, "planet", "universe")

	// This will be our reference expected value.
	refTagsets := []*logtags.Buffer{
		logtags.SingleTagBuffer("planet", "universe"),
		logtags.SingleTagBuffer("foo", "123").Add("x", "456").Add("bar", ""),
	}

	// Construct the error object.
	origErr := errors.New("hello")
	decoratedErr := errors.WithContextTags(origErr, ctx)
	decoratedErr = errors.WithContextTags(decoratedErr, ctx2)

	theTest := func(tt testutils.T, err error) {
		// Ensure that the original error object can be found.
		// This test that the cause interface works properly.
		tt.Check(markers.Is(err, origErr))

		// Ensure that the decorated error can be found.
		// This checks that the wrapper identity
		// is properly preserved across the network.
		tt.Check(markers.Is(err, decoratedErr))

		// Check that the message is preserved.
		tt.CheckEqual(err.Error(), "hello")

		// Check that the tag pairs are preserved.
		tagsets := contexttags.GetContextTags(err)
		if len(tagsets) != len(refTagsets) {
			tt.CheckEqual(len(tagsets), len(refTagsets))
		} else {
			for i, actualB := range tagsets {
				refB := refTagsets[i]
				tt.CheckDeepEqual(actualB.Get(), refB.Get())
			}
		}
	}

	tt.Run("local", func(tt testutils.T) { theTest(tt, decoratedErr) })

	enc := errbase.EncodeError(context.Background(), decoratedErr)
	newErr := errbase.DecodeError(context.Background(), enc)

	tt.Run("remote", func(tt testutils.T) { theTest(tt, newErr) })
}
