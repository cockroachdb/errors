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

package contexttags

import (
	"fmt"

	"github.com/cockroachdb/errors/errbase"
	"github.com/cockroachdb/errors/errorspb"
	"github.com/cockroachdb/logtags"
	"github.com/gogo/protobuf/proto"
)

type withContext struct {
	cause error
	// tags stores the context k/v pairs.
	// The errors library only gives access to the string representation
	// of the value part. This is because the network encoding of
	// a withContext instance only stores the string.
	tags *logtags.Buffer
}

var _ error = (*withContext)(nil)

// withContext is an error. The original error message is preserved.
func (w *withContext) Error() string { return w.cause.Error() }

// the cause is reachable.
func (w *withContext) Cause() error  { return w.cause }
func (w *withContext) Unwrap() error { return w.cause }

// Printing a withContext reveals the tags.
func (w *withContext) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			fmt.Fprintf(s, "%+v", w.cause)
			if w.tags != nil {
				fmt.Fprintf(s, "\n-- context: %s", w.tags.String())
			}
			return
		}
		fallthrough
	case 's', 'q':
		errbase.FormatError(s, verb, w.cause)
	}
}

func encodeWithContext(err error) (string, []string, proto.Message) {
	w := err.(*withContext)
	p := &errorspb.TagsPayload{}
	for _, t := range w.tags.Get() {
		p.Tags = append(p.Tags, errorspb.TagPayload{Tag: t.Key(), Value: t.ValueStr()})
	}
	return "", nil, p
}

func decodeWithContext(cause error, _ string, _ []string, payload proto.Message) error {
	m, ok := payload.(*errorspb.TagsPayload)
	if !ok {
		// If this ever happens, this means some version of the library
		// (presumably future) changed the payload type, and we're
		// receiving this here. In this case, give up and let
		// DecodeError use the opaque type.
		return nil
	}
	if len(m.Tags) == 0 {
		// There are no tags stored. Either there are no tags stored, or
		// we received some new version of the protobuf message which does
		// things differently. Again, use the opaque type.
		return nil
	}
	// Convert the k/v pairs.
	var b *logtags.Buffer
	for _, t := range m.Tags {
		b = b.Add(t.Tag, t.Value)
	}
	return &withContext{cause: cause, tags: b}
}

func init() {
	errbase.RegisterWrapperEncoder(errbase.GetTypeKey((*withContext)(nil)), encodeWithContext)
	errbase.RegisterWrapperDecoder(errbase.GetTypeKey((*withContext)(nil)), decodeWithContext)
}
