# cockroachdb/errors: an error library with network portability

After the discussion in https://github.com/cockroachdb/cockroach/pull/36987
and https://github.com/cockroachdb/cockroach/pull/37121

| Feature                                                                                         | Go's <1.13 `errors` | Go 1.13 `errors` (`xerrors`) | `github.com/pkg/errors` | `cockroachdb/errors` |
|-------------------------------------------------------------------------------------------------|---------------------|------------------------------|-------------------------|----------------------|
| error constructors (`New`, `Errorf` etc)                                                        | ✔                   | ✔                            | ✔                       | ✔                    |
| error causes (`Cause` / `Unwrap`)                                                               |                     | ✔                            | ✔                       | ✔                    |
| cause barriers (`Opaque` / `Handled`)                                                           |                     | ✔                            |                         | ✔                    |
| `errors.Is()`                                                                                   |                     | ✔                            |                         | ✔                    |
| standard wrappers with stack trace capture                                                      |                     |                              | ✔                       | ✔                    |
| **transparent protobuf encode/decode**                                                          |                     |                              |                         | ✔                    |
| **`errors.Is()` recognizes errors across the network**                                          |                     |                              |                         | ✔                    |
| **comprehensive support for PII-free reportable strings**                                       |                     |                              |                         | ✔                    |
| support for both `Cause()` and `Unwrap()` [go#31778](https://github.com/golang/go/issues/31778) |                     |                              |                         | ✔                    |
| standard error reports to Sentry.io                                                             |                     |                              |                         | ✔                    |
| wrappers to denote assertion failures                                                           |                     |                              |                         | ✔                    |
| wrappers with issue tracker references                                                          |                     |                              |                         | ✔                    |
| wrappers for user-facing hints and details                                                      |                     |                              |                         | ✔                    |
| wrappers to attach secondary causes                                                             |                     |                              |                         | ✔                    |
| `errors.As()`                                                                                   |                     | ✔                            |                         | (in construction)    |
| `errors.FormatError()`, `Formatter`, `Printer`                                                  |                     | ✔                            |                         | (in construction)    |
