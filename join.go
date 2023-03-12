package errors

import (
	"fmt"
)

// Join returns an error that wraps the given errors.
// Any nil error values are discarded.
// Join returns nil if errs contains no non-nil values.
// The error formats as the concatenation of the strings obtained
// by calling the Error method of each element of errs, with a newline
// between each string.
func Join(errs ...error) error {
	n := 0
	for _, err := range errs {
		if err != nil {
			n++
		}
	}
	if n == 0 {
		return nil
	}
	e := &joinError{
		errs: make([]error, 0, n),
	}
	for _, err := range errs {
		if err != nil {
			e.errs = append(e.errs, err)
		}
	}
	return e
}

type joinError struct {
	errs []error
}

func (e *joinError) Error() string {
	var b []byte
	for i, err := range e.errs {
		if i > 0 {
			b = append(b, '\n')
		}
		b = append(b, err.Error()...)
	}
	return string(b)
}

func (e *joinError) Unwrap() []error {
	return e.errs
}

func (e *joinError) Format(w fmt.State, verb rune) {
	for i, err := range e.Unwrap() {
		if i > 0 {
			fmt.Fprint(w, "\n")
		}
		fmt.Fprintf(w, "[%d]:\n  ", i)
		s := fmt.Sprintf(fmt.FormatString(w, verb), err)
		s = strings.Join(strings.Split(s, "\n"), "\n  ")
		fmt.Fprint(w, s)
	}
}
