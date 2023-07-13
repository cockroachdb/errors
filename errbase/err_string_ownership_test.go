package errbase_test

import (
	"context"
	goErr "errors"
	"fmt"
	"testing"

	"github.com/cockroachdb/errors/errbase"
	"github.com/cockroachdb/errors/errorspb"
	"github.com/cockroachdb/errors/testutils"
)

func genEncoded(ownsErrorString bool) errorspb.EncodedError {
	return errorspb.EncodedError{
		Error: &errorspb.EncodedError_Wrapper{
			Wrapper: &errorspb.EncodedWrapper{
				Cause: errorspb.EncodedError{
					Error: &errorspb.EncodedError_Leaf{
						Leaf: &errorspb.EncodedErrorLeaf{
							Message: "leaf-error-msg",
						},
					},
				},
				MessagePrefix:          "wrapper-error-msg: leaf-error-msg: extra info",
				Details:                errorspb.EncodedErrorDetails{},
				WrapperOwnsErrorString: ownsErrorString,
			},
		},
	}
}

func TestDecodeOldVersion(t *testing.T) {
	tt := testutils.T{T: t}

	errOldEncoded := genEncoded(false)
	errOldDecoded := errbase.DecodeError(context.Background(), errOldEncoded)
	// Ensure that we will continue to just concat leaf after wrapper
	// with older errors for backward compatibility.
	tt.CheckEqual(errOldDecoded.Error(), "wrapper-error-msg: leaf-error-msg: extra info: leaf-error-msg")

	// Check to ensure that when flag is present, we interpret things correctly.
	errNewEncoded := genEncoded(true)
	errNewDecoded := errbase.DecodeError(context.Background(), errNewEncoded)
	tt.CheckEqual(errNewDecoded.Error(), "wrapper-error-msg: leaf-error-msg: extra info")
}

func TestEncodeDecodeNewVersion(t *testing.T) {
	tt := testutils.T{T: t}
	errNewEncoded := errbase.EncodeError(
		context.Background(),
		fmt.Errorf(
			"wrapper-error-msg: %w: extra info",
			goErr.New("leaf-error-msg"),
		),
	)

	errNew := errorspb.EncodedError{
		Error: &errorspb.EncodedError_Wrapper{
			Wrapper: &errorspb.EncodedWrapper{
				Cause: errorspb.EncodedError{
					Error: &errorspb.EncodedError_Leaf{
						Leaf: &errorspb.EncodedErrorLeaf{
							Message: "leaf-error-msg",
							Details: errorspb.EncodedErrorDetails{
								OriginalTypeName:  "errors/*errors.errorString",
								ErrorTypeMark:     errorspb.ErrorTypeMark{FamilyName: "errors/*errors.errorString", Extension: ""},
								ReportablePayload: nil,
								FullDetails:       nil,
							},
						},
					},
				},
				MessagePrefix: "wrapper-error-msg: leaf-error-msg: extra info",
				Details: errorspb.EncodedErrorDetails{
					OriginalTypeName:  "fmt/*fmt.wrapError",
					ErrorTypeMark:     errorspb.ErrorTypeMark{FamilyName: "fmt/*fmt.wrapError", Extension: ""},
					ReportablePayload: nil,
					FullDetails:       nil,
				},
				WrapperOwnsErrorString: true,
			},
		},
	}

	tt.CheckDeepEqual(errNewEncoded, errNew)
	newErr := errbase.DecodeError(context.Background(), errNew)

	// New version correctly decodes error
	tt.CheckEqual(newErr.Error(), "wrapper-error-msg: leaf-error-msg: extra info")
}
