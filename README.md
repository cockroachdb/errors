# cockroachdb/errors: Go errors with network portability

This library was designed in the fashion discussed in
https://github.com/cockroachdb/cockroach/pull/36987 and
https://github.com/cockroachdb/cockroach/pull/37121

| Feature                                                                                               | Go's <1.13 `errors` | `github.com/pkg/errors` | Go 1.13 `errors`/`xerrors` | `cockroachdb/errors` |
|-------------------------------------------------------------------------------------------------------|---------------------|-------------------------|----------------------------|----------------------|
| error constructors (`New`, `Errorf` etc)                                                              | ✔                   | ✔                       | ✔                          | ✔                    |
| error causes (`Cause` / `Unwrap`)                                                                     |                     | ✔                       | ✔                          | ✔                    |
| cause barriers (`Opaque` / `Handled`)                                                                 |                     |                         | ✔                          | ✔                    |
| `errors.Is()`                                                                                         |                     |                         | ✔                          | ✔                    |
| standard wrappers with efficient stack trace capture                                                  |                     | ✔                       |                            | ✔                    |
| **transparent protobuf encode/decode with forward compatibility**                                     |                     |                         |                            | ✔                    |
| **`errors.Is()` recognizes errors across the network**                                                |                     |                         |                            | ✔                    |
| **comprehensive support for PII-free reportable strings**                                             |                     |                         |                            | ✔                    |
| support for both `Cause()` and `Unwrap()` [go#31778](https://github.com/golang/go/issues/31778)       |                     |                         |                            | ✔                    |
| standard error reports to Sentry.io                                                                   |                     |                         |                            | ✔                    |
| wrappers to denote assertion failures                                                                 |                     |                         |                            | ✔                    |
| wrappers with issue tracker references                                                                |                     |                         |                            | ✔                    |
| wrappers for user-facing hints and details                                                            |                     |                         |                            | ✔                    |
| wrappers to attach secondary causes                                                                   |                     |                         |                            | ✔                    |
| wrappers to attach [`logtags`](https://github.com/cockroachdb/logtags) details from `context.Context` |                     |                         |                            | ✔                    |
| `errors.As()`                                                                                         |                     |                         | ✔                          | (under construction)    |
| `errors.FormatError()`, `Formatter`, `Printer`                                                        |                     |                         | ✔                          | (under construction)    |

"Forward compatibility" above refers to the ability of this library to
recognize and properly handle network communication of error types it
does not know about, for example when a more recent version of a
software package sends a new error object to another system running an
older version of the package.

## How to use

- construct errors with `errors.New()`, etc as usual, but also see the other [error leaf constructors](#Available-error-leaves) below.
- wrap errors with `errors.Wrap()` as usual, but also see the [other wrappers](#Available-wrapper-constructors) below.
- test error identity with `errors.Is()` as usual.
  **Unique in this library**: this works even if the error has traversed the network!
  Also, `errors.IsAny()` to recognize two or more reference errors.
- access error causes with `errors.UnwrapOnce()` / `errors.UnwrapAll()` (note: `errors.Cause()` and `errors.Unwrap()` also provided for compatibility with other error packages).
- encode/decode errors to protobuf with `errors.EncodeError()` / `errors.DecodeError()`.
- extract **PII-free safe details** with `errors.GetSafeDetails()`.
- extract human-facing hints and details with `errors.GetAllHints()`/`errors.GetAllDetails()` or `errors.FlattenHints()`/`errors.FlattenDetails()`.
- produce detailed Sentry.io reports with `errors.BuildSentryReport()` / `errors.ReportError()`.
- implement your own error leaf types and wrapper types:
  - implement the `error` and `errors.Wrapper` interfaces as usual.
  - register encode/decode functions: call `errors.Register{Leaf,Wrapper}{Encoder,Decoder}()` in a `init()` function in your package.
  - see the sub-package `exthttp` for an example.

## What comes out of an error?

| Error detail                                                    | `Error()` and format `%s`/`%q`/`%v` | format `%+v` | `GetSafeDetails()`            | Sentry report via `ReportError()` |
|-----------------------------------------------------------------|-------------------------------------|--------------|-------------------------------|-----------------------------------|
| main message, eg `New()`                                        | visible                             | visible      | redacted                      | redacted                          |
| wrap prefix, eg `WithMessage()`                                 | visible (as prefix)                 | visible      | redacted                      | redacted                          |
| stack trace, eg `WithStack()`                                   | not visible                         | visible      | yes                           | full                              |
| hint , eg `WithHint()`                                          | not visible                         | visible      | no                            | type only                         |
| detail, eg `WithDetail()`                                       | not visible                         | visible      | no                            | type only                         |
| assertion failure annotation, eg `WithAssertionFailure()`       | not visible                         | visible      | no                            | type only                         |
| issue links, eg `WithIssueLink()`, `UnimplementedError()`       | not visible                         | visible      | yes                           | full                              |
| safe details, eg `WithSafeDetails()`                            | not visible                         | visible      | yes                           | full                              |
| telemetry keys, eg. `WithTelemetryKey()`                        | not visible                         | visible      | yes                           | full                              |
| secondary errors, eg. `WithSecondaryError()`, `CombineErrors()` | not visible                         | visible      | redacted, recursively         | redacted, recursively             |
| barrier origins, eg. `Handled()`                                | not visible                         | visible      | redacted, recursively         | redacted, recursively             |
| error domain, eg. `WithDomain()`                                | not visible                         | visible      | yes                           | full                              |
| context tags, eg. `WithContextTags()`                           | not visible                         | visible      | keys visible, values redacted | keys visible, values redacted     |

## Available error leaves

- `New(string) error`, `Newf(string, ...interface{}) error`, `Errorf(string, ...interface{}) error`: leaf errors with message
  - **when to use: common error cases.**
  - what it does: also captures the stack trace at point of call and redacts the provided message for safe reporting.
  - how to access the detail: `Error()`, regular Go formatting. Details redacted in Sentry report.
  - see also: Section [Error composition](#Error-composition-summary) below. `errors.NewWithDepth()` variants to customize at which call depth the stack trace is captured.

- `AssertionFailedf(string, ...interface{}) error`, `NewAssertionFailureWithWrappedErrf(error, string, ...interface{}) error`: signals an assertion failure / programming error.
  - **when to use: when an invariant is violated; when an unreachable code path is reached.**
  - what it does: also captures the stack trace at point of call, redacts the provided strings for safe reporting, prepares a hint to inform a human user.
  - how to access the detail: `IsAssertionFailure()`/`HasAssertionFailure()`, format with `%+v`, Safe details included in Sentry reports.
  - see also: Section [Error composition](#Error-composition-summary) below. `errors.AssertionFailedWithDepthf()` variant to customize at which call depth the stack trace is captured.

- `Handled(error) error`, `Opaque(error) error`, `HandledWithMessage(error, string) error`: captures an error cause but make it invisible to `Unwrap()` / `Is()`.
  - **when to use: when a new error occurs while handling an error, and the original error must be "hidden".**
  - what it does: captures the cause in a hidden field. The error message is preserved unless the `...WithMessage()` variant is used.
  - how to access the detail: format with `%+v`, redacted details reported in Sentry reports.

- `UnimplementedError(IssueLink, string) error`: captures a message string and a URL reference to an external resource to denote a feature that was not yet implemented.
  - **when to use: to inform (human) users that some feature is not implemented yet and refer them to some external resource.**
  - what it does: captures the message, URL and detail in a wrapper. The URL and detail are considered safe for reporting.
  - how to access the detail: `errors.GetAllHints()`, `errors.FlattenHints()`, format with `%+v`, URL and detail included in Sentry report (not the message).
  - see also: `errors.WithIssueLink()` below for errors that are not specifically about unimplemented features.

## Available wrapper constructors

All wrapper constructors can be applied safely to a `nil` `error`:
they behave as no-ops in this case:

```go
// The following:
// if err := foo(); err != nil {
//    return errors.Wrap(err, "foo")
// }
// return nil
//
// is not needed. Instead, you can use this:
return errors.Wrap(foo())
```

- `Wrap(error, string) error`, `Wrapf(error, string, ...interface{}) error`:
  - **when to use: on error return paths.**
  - what it does: combines `WithMessage()`, `WithStack()`, `WithSafeDetails()`.
  - how to access the details: `Error()`, regular Go formatting. Details redacted in Sentry report.
  - see also: Section [Error composition](#Error-composition-summary) below. `WrapWithDepth()` variants to customize at which depth the stack trace is captured.

- `WithSecondaryError(error, error) error`: annotate an error with a secondary error.
  - **when to use: when an additional error occurs in the code that is handling a primary error.** Consider using `errors.CombineErrors()` instead (see below).
  - what it does: it captures the secondary error but hides it from `errors.Is()`.
  - how to access the detail: format with `%+v`, redacted recursively in Sentry reports.
  - see also: `errors.CombineErrors()`

- `CombineErrors(error, error) error`: combines two errors into one.
  - **when to use: when two operations occur concurrently and either can return an error, and only one final error must be returned.**
  - what it does: returns either of its arguments if the other is `nil`, otherwise calls `WithSecondaryError()`.
  - how to access the detail: see `WithSecondaryError()` above.

- `Mark(error, error) error`: gives the identity of one error to another error.
  - **when to use: when a caller expects to recognize a sentinel error with `errors.Is()` but the callee provides a diversity of error messages.**
  - what it does: it overrides the "error mark" used internally by `errors.Is()`.
  - how to access the detail: format with `%+v`, Sentry reports.

- `WithStack(error) error`: annotate with stack trace
  - **when to use:** usually not needed, use `errors.Wrap()`/`errors.Wrapf()` instead.

    **Special cases:**

    - when returning a sentinel, for example:

      ```go
      var myErr = errors.New("foo")

      func myFunc() error {
        if ... {
           return errors.WithStack(myErr)
        }
      }
      ```

    - on error return paths, when not trivial but also not warranting a wrap. For example:

      ```go
      err := foo()
      if err != nil {
        doSomething()
        if !somecond {
           return errors.WithStack(err)
        }
      }
        ```

  - what it does: captures (efficiently) a stack trace.
  - how to access the details: format with `%+v`, `errors.GetSafeDetails()`, Sentry reports. The stack trace is considered safe for reporting.
  - see also: `WithStackDepth()` to customize the call depth at which the stack trace is captured.

- `WithSafeDetails(error, string, ...interface{}) error`: safe details for reporting.
  - when to use: probably never. Use `errors.Wrap()`/`errors.Wrapf()` instead.
  - what it does: saves some strings for safe reporting.
  - how to access the detail: format with `%+v`, `errors.GetSafeDetails()`, Sentry report.

- `WithMessage(error, string) error`, `WithMessagef(error, string, ...interface{}) error`: message prefix.
  - when to use: probably never. Use `errors.Wrap()`/`errors.Wrapf()` instead.
  - what it does: adds a message prefix.
  - how to access the detail: `Error()`, regular Go formatting. Not included in Sentry reports.

- `WithDetail(error, string) error`, `WithDetailf(error, string, ...interface{}) error`, user-facing detail with contextual information.
  - **when to use: need to embark a message string to output when the error is presented to a human.**
  - what it does: captures detail strings.
  - how to access the detail: `errors.GetAllDetails()`, `errors.FlattenDetails()` (all details are preserved), format with `%+v`.

- `WithHint(error, string) error`, `WithHintf(error, string, ...interface{}) error`: user-facing detail with suggestion for action to take.
  - **when to use: need to embark a message string to output when the error is presented to a human.**
  - what it does: captures hint strings.
  - how to access the detail: `errors.GetAllHints()`, `errors.FlattenHints()` (hints are de-duplicated), format with `%+v`.

- `WithIssueLink(error, IssueLink) error`: annotate an error with an URL and arbitrary string.
  - **when to use: to refer (human) users to some external resources.**
  - what it does: captures the URL and detail in a wrapper. Both are considered safe for reporting.
  - how to access the detail: `errors.GetAllHints()`, `errors.FlattenHints()`,  `errors.GetSafeDetails()`, format with `%+v`, Sentry report.
  - see also: `errors.UnimplementedError()` to construct leaves (see previous section).

- `WithTelemetry(error, string) error`: annotate an error with a key suitable for telemetry.
  - **when to use: to gather strings during error handling, for capture in the telemetry sub-system of a server package.**
  - what it does: captures the string. The telemetry key is considered safe for reporting.
  - how to access the detail: `errors.GetTelemetryKeys()`,  `errors.GetSafeDetails()`, format with `%+v`, Sentry report.

- `WithDomain(error, Domain) error`, `HandledInDomain(error, Domain) error`, `HandledInDomainWithMessage(error, Domain, string) error` **(experimental)**: annotate an error with an origin package.
  - **when to use: at package boundaries.**
  - what it does: captures the identity of the error domain. Can be asserted with `errors.EnsureNotInDomain()`, `errors.NotInDomain()`.
  - how to access the detail: format with `%+v`, Sentry report.

- `WithAssertionFailure(error) error`: annotate an error as being an assertion failure.
  - when to use: probably never. Use `errors.AssertionFailedf()` and variants.
  - what it does: wraps the error with a special type. Triggers an auto-generated hint.
  - how to access the detail: `IsAssertionFailure()`/`HasAssertionFailure()`, `errors.GetAllHints()`, `errors.FlattenHints()`, format with `%+v`, Sentry report.

- `WithContextTags(error, context.Context) error`: annotate an error with the k/v pairs attached to a `context.Context` instance with the [`logtags`](https://github.com/cockroachdb/logtags) package.
  - **when to use: when capturing/producing an error and a `context.Context` is available.**
  - what it does: it captures the `logtags.Buffer` object in the wrapper.
  - how to access the detail: `errors.GetContextTags()`, format with `%+v`, Sentry reports.

## Error composition (summary)

| Constructor                        | Composes                                                                                         |
|------------------------------------|--------------------------------------------------------------------------------------------------|
| `New`                              | `NewWithDepth` (see below)                                                                       |
| `Errorf`                           | = `Newf`                                                                                         |
| `Newf`                             | `NewWithDepthf` (see below)                                                                      |
| `WithMessage`                      | = `pkgErr.WithMessage`                                                                           |
| `Wrap`                             | `WrapWithDepth` (see below)                                                                      |
| `Wrapf`                            | `WrapWithDepthf` (see below)                                                                     |
| `AssertionFailed`                  | `AssertionFailedWithDepthf` (see below)                                                          |
| `NewWithDepth`                     | `goErr.New` + `WithStackDepth` (see below)                                                       |
| `NewWithDepthf`                    | `fmt.Errorf` + `WithSafeDetails` + `WithStackDepth`                                              |
| `WithMessagef`                     | `pkgErr.WithMessagef` + `WithSafeDetails`                                                        |
| `WrapWithDepth`                    | `WithMessage` + `WithStackDepth`                                                                 |
| `WrapWithDepthf`                   | `WithMessage` + `WithStackDepth` + `WithSafeDetails`                                             |
| `AssertionFailedWithDepthf`        | `fmt.Errorf` + `WithStackDepth` + `WithSafeDetails` + `WithAssertionFailure`                     |
| `NewAssertionErrorWithWrappedErrf` | `HandledWithMessagef` (barrier) + `WithStackDepth` + `WithSafeDetails` +  `WithAssertionFailure` |

## API (not constructing error objects)

```go
// Access causes.
func UnwrapAll(err error) error
func UnwrapOnce(err error) error
func Cause(err error) error // compatibility
func Unwrap(err error) error // compatibility
type Wrapper interface { ... } // compatibility

// Identify errors.
func Is(err, reference error) bool
func IsAny(err error, references ...error) bool
func If(err error, pred func(err error) (interface{}, bool)) (interface{}, bool)

// Encode/decode errors.
type EncodedError // this is protobuf-encodable
func EncodeError(ctx context.Context, err error) EncodedError
func DecodeError(ctx context.Context, enc EncodedError) error

// Register encode/decode functions for custom/new error types.
func RegisterLeafDecoder(typeName TypeKey, decoder LeafDecoder)
func RegisterLeafEncoder(typeName TypeKey, encoder LeafEncoder)
func RegisterWrapperDecoder(typeName TypeKey, decoder WrapperDecoder)
func RegisterWrapperEncoder(typeName TypeKey, encoder WrapperEncoder)
type LeafEncoder = func(ctx context.Context, err error) (msg string, safeDetails []string, payload proto.Message)
type LeafDecoder = func(ctx context.Context, msg string, safeDetails []string, payload proto.Message) error
type WrapperEncoder = func(ctx context.Context, err error) (msgPrefix string, safeDetails []string, payload proto.Message)
type WrapperDecoder = func(ctx context.Context, cause error, msgPrefix string, safeDetails []string, payload proto.Message) error

// Sentry reports.
func BuildSentryReport(err error) (string, []raven.Interface, map[string]interface{})
func ReportError(err error) (string, error)

// Stack trace captures.
func GetOneLineSource(err error) (file string, line int, fn string, ok bool)
type ReportableStackTrace = raven.StackTrace
func GetReportableStackTrace(err error) *ReportableStackTrace

// Safe (PII-free) details.
type SafeDetailPayload struct { ... }
func GetAllSafeDetails(err error) []SafeDetailPayload
func GetSafeDetails(err error) (payload SafeDetailPayload)
type SafeMessager interface { ... }
func Safe(v interface{}) SafeMessager
func Redact(r interface{}) string

// Assertion failures.
func HasAssertionFailure(err error) bool
func IsAssertionFailure(err error) bool

// User-facing details and hints.
func GetAllDetails(err error) []string
func FlattenDetails(err error) string
func GetAllHints(err error) []string
func FlattenHints(err error) string

// Issue links / URL wrappers.
func HasIssueLink(err error) bool
func IsIssueLink(err error) bool
func GetAllIssueLinks(err error) (issues []IssueLink)

// Unimplemented errors.
func HasUnimplementedError(err error) bool
func IsUnimplementedError(err error) bool

// Telemetry keys.
func GetTelemetryKeys(err error) []string

// Domain errors.
type Domain
const NoDomain Domain
func GetDomain(err error) Domain
func NamedDomain(domainName string) Domain
func PackageDomain() Domain
func PackageDomainAtDepth(depth int) Domain
func EnsureNotInDomain(err error, constructor DomainOverrideFn, forbiddenDomains ...Domain) error
func NotInDomain(err error, doms ...Domain) bool

// Context tags.
func GetContextTags(err error) []*logtags.Buffer
```
