module github.com/cockroachdb/errors

go 1.13

require (
	github.com/cockroachdb/logtags v0.0.0-20190617123548-eb05cc24525f
	github.com/getsentry/raven-go v0.2.0
	github.com/gogo/protobuf v1.3.0
	github.com/kr/pretty v0.1.0
	github.com/pkg/errors v0.8.1
)

replace github.com/getsentry/raven-go => github.com/tooolbox/raven-go v0.2.1-0.20191013052125-7826697885e9
