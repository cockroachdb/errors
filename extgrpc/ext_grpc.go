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

package extgrpc

//go:generate protoc ext_grpc.proto --gogofaster_out=.

import (
	"context"
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/errors/errbase"
	"github.com/cockroachdb/errors/markers"
	"github.com/gogo/protobuf/proto"

	"google.golang.org/grpc/codes"
)

// This file demonstrates how to add a wrapper type not otherwise
// known to the rest of the library.

// withGrpcCode is our wrapper type.
type withGrpcCode struct {
	cause error
	code  codes.Code
}

// WrapWithGrpcCode adds a Grpc code to an existing error.
func WrapWithGrpcCode(err error, code codes.Code) error {
	if err == nil {
		return nil
	}
	return &withGrpcCode{cause: err, code: code}
}

// GetGrpcCode retrieves the Grpc code from a stack of causes.
func GetGrpcCode(err error) codes.Code {
	if err == nil {
		return codes.OK
	}
	if v, ok := markers.If(err, func(err error) (interface{}, bool) {
		if w, ok := err.(*withGrpcCode); ok {
			return w.code, true
		}
		return nil, false
	}); ok {
		return v.(codes.Code)
	}
	return codes.Unknown
}

// it's an error.
func (w *withGrpcCode) Error() string { return w.cause.Error() }

// it's also a wrapper.
func (w *withGrpcCode) Cause() error  { return w.cause }
func (w *withGrpcCode) Unwrap() error { return w.cause }

// it knows how to format itself.
func (w *withGrpcCode) Format(s fmt.State, verb rune) { errors.FormatError(w, s, verb) }
func (w *withGrpcCode) FormatError(p errors.Printer) (next error) {
	if p.Detail() {
		p.Printf("gRPC code: %s", w.code.String())
	}
	return w.cause
}

// it's an encodable error.
func encodeWithGrpcCode(_ context.Context, err error) (string, []string, proto.Message) {
	w := err.(*withGrpcCode)
	details := []string{fmt.Sprintf("gRPC %d", w.code)}
	payload := &EncodedGrpcCode{Code: uint32(w.code)}
	return "", details, payload
}

// it's a decodable error.
func decodeWithGrpcCode(
	_ context.Context, cause error, _ string, _ []string, payload proto.Message,
) error {
	wp := payload.(*EncodedGrpcCode)
	return &withGrpcCode{cause: cause, code: codes.Code(wp.Code)}
}

func init() {
	errbase.RegisterWrapperEncoder(errbase.GetTypeKey((*withGrpcCode)(nil)), encodeWithGrpcCode)
	errbase.RegisterWrapperDecoder(errbase.GetTypeKey((*withGrpcCode)(nil)), decodeWithGrpcCode)
}
