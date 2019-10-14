package middleware

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/errors/extgrpc"
	"github.com/gogo/status"

	"google.golang.org/grpc"
)

func UnaryServerInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {

	resp, err := handler(ctx, req)
	if err == nil {
		return resp, err
	}

	st, ok := status.FromError(err)
	if !ok {
		code := extgrpc.GetGrpcCode(err)
		st = status.New(code, err.Error())
		enc := errors.EncodeError(ctx, err)
		st, err = st.WithDetails(&enc)
		if err != nil {
			panic(err) // Programmer error
		}
	}

	return resp, st.Err()
}
