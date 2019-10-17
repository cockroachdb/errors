package status

import (
	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/errors/extgrpc"

	"google.golang.org/grpc/codes"
)

func Error(c codes.Code, msg string) error {
	return extgrpc.WrapWithGrpcCode(errors.New(msg), c)
}

func WrapErr(c codes.Code, msg string, err error) error {
	return extgrpc.WrapWithGrpcCode(errors.WrapWithDepth(1, err, msg), c)
}

func Code(err error) codes.Code {
	return extgrpc.GetGrpcCode(err)
}
