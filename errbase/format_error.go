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

// This file is forked and modified from golang.org/x/xerrors,
// at commit 3ee3066db522c6628d440a3a91c4abdd7f5ef22f (2019-05-10).
// From the original code:
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// Changes specific to this fork marked as inline comments.

package errbase

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strconv"

	pkgErr "github.com/pkg/errors"
)

// FormatError formats an error according to s and verb.
//
// If the error implements errors.Formatter, FormatError calls its
// FormatError method of f with an errors.Printer configured according
// to s and verb, and writes the result to s.
//
// Otherwise, if it is a wrapper, FormatError prints out its error prefix,
// then recurses on its cause.
//
// Otherwise, its Error() text is printed.
func FormatError(err error, s fmt.State, verb rune) {
	// Assuming this function is only called from the Format method, and given
	// that FormatError takes precedence over Format, it cannot be called from
	// any package that supports errors.Formatter. It is therefore safe to
	// disregard that State may be a specific printer implementation and use one
	// of our choice instead.

	p := state{State: s}

	switch {
	case verb == 'v' && s.Flag('#'):
		if stringer, ok := err.(fmt.GoStringer); ok {
			io.WriteString(&p.buf, stringer.GoString())
			p.finishDisplay(verb)
			return
		}
		// Not a stringer. Proceed as if it were %v.
		fallthrough

	case verb == 's' || verb == 'q' ||
		verb == 'x' || verb == 'X' || verb == 'v':
		// In all these cases, the first line in the output is the
		// complete error message.
		//
		// Use an intermediate buffer because there may be alignment
		// instructions to obey in the final rendering or
		// quotes to add (for %q).
		p.buf.Write([]byte(err.Error()))

		// In all the cases but %+v, we finish here.
		if verb != 'v' || !s.Flag('+') {
			p.finishDisplay(verb)
			return
		}
		// %+v was specified. We'll finish this below.

	default:
		// Unknown verb. Do like fmt.Printf and tell the user we're
		// confused.
		p.buf.WriteString("%!")
		p.buf.WriteRune(verb)
		p.buf.WriteByte('(')
		switch {
		case err != nil:
			p.buf.WriteString(reflect.TypeOf(err).String())
		default:
			p.buf.WriteString("<nil>")
		}
		p.buf.WriteByte(')')
		io.Copy(s, &p.buf)
		return
	}

	// Here we are going to format as per %+v, into p.buf.  We really
	// want to print the innermost (root cause) error first, then the
	// layers of wrapping from innermost to outermost. This requires a
	// post-order traversal. Since we have a linked list, the best we
	// can do is a recursion.
	p.formatRecursive(err, true /* isFirst */)

	// We're done formatting.
	p.finishDisplay(verb)
}

// formatRecursive performs a post-order traversal to
// prints errors from innermost to outermost.
func (s *state) formatRecursive(err error, isFirst bool) {
	cause := UnwrapOnce(err)
	if cause != nil {
		// Recurse first.
		s.formatRecursive(cause, false /*isFirst*/)
	}

	// We've printed all the cause details so far. Now print out the
	// details for this wrapper.
	fmt.Fprintf(&s.buf, "\n- (%T:)", err)

	s.needSpace = true
	s.inDetail = false
	s.needNewline = false
	s.multiLine = false

	switch v := err.(type) {
	case Formatter:
		_ = v.FormatError((*printer)(s))
	case fmt.Formatter:
		// We can only use a fmt.Formatter when both the following
		// conditions are true:
		// - when it is the leaf error, because a fmt.Formatter
		//   on a wrapper also recurses.
		// - when it is not the outermost wrapper, because
		//   the Format() method is likely to be calling FormatError()
		//   to do its job and we want to avoid an infinite recursion.
		if !isFirst && cause == nil {
			v.Format(s, 'v')
			if st, ok := err.(StackTraceProvider); ok {
				// This is likely a leaf error from github/pkg/errors.
				// The thing probably printed its stack trace on its own.
				// We'll subsequently simplify stack traces in wrappers.
				s.lastStack = st.StackTrace()
			}
		} else {
			s.formatSimple(err, cause)
		}
	default:
		// If the error did not implement errors.Formatter nor
		// fmt.Formatter, but it is a wrapper, still attempt best effort:
		// print what we can at this level.
		s.formatSimple(err, cause)
	}
}

func (s *state) formatSimple(err, cause error) {
	var pref string
	if cause != nil {
		pref = extractPrefix(err, cause)
	} else {
		pref = err.Error()
	}
	if len(pref) > 0 {
		s.Write([]byte(pref))
	}
	// If there's an embedded stack trace, print it.
	// This will get either a stack from pkg/errors, or ours.
	if st, ok := err.(StackTraceProvider); ok {
		if s.multiLine {
			s.Write([]byte("\n-- stack trace:"))
		}
		newStack, elided := ElideSharedStackTraceSuffix(s.lastStack, st.StackTrace())
		s.lastStack = newStack
		newStack.Format(s, 'v')
		if elided {
			s.Write([]byte("\n[...same entries as above...]"))
		}
	}
}

// finishDisplay renders the buffer in state into the fmt.State.
func (p *state) finishDisplay(verb rune) {
	width, okW := p.Width()
	prec, okP := p.Precision()

	// If `direct` is set to false, then the buffer is always
	// passed through fmt.Printf regardless of the width and alignment
	// settings. This is important or e.g. %q where quotes must be added
	// in any case.
	// If `direct` is set to true, then the detour via
	// fmt.Printf only occurs if there is a width or alignment
	// specifier.
	direct := verb == 'v' || verb == 's'

	if !direct || (okW && width > 0) || okP {
		// Construct format string from State s.
		format := []byte{'%'}
		if p.Flag('-') {
			format = append(format, '-')
		}
		if p.Flag('+') {
			format = append(format, '+')
		}
		if p.Flag(' ') {
			format = append(format, ' ')
		}
		if okW {
			format = strconv.AppendInt(format, int64(width), 10)
		}
		if okP {
			format = append(format, '.')
			format = strconv.AppendInt(format, int64(prec), 10)
		}
		format = append(format, string(verb)...)
		fmt.Fprintf(p.State, string(format), p.buf.String())
	} else {
		io.Copy(p.State, &p.buf)
	}
}

var detailSep = []byte("\n    ")

// state tracks error printing state. It implements fmt.State.
type state struct {
	fmt.State
	buf bytes.Buffer

	lastStack   StackTrace
	inDetail    bool
	needSpace   bool
	needNewline bool
	multiLine   bool
}

func (s *state) Write(b []byte) (n int, err error) {
	if len(b) == 0 {
		return 0, nil
	}
	k := 0
	for i, c := range b {
		if c == '\n' {
			s.buf.Write(b[k:i])
			k = i + 1
			s.needNewline = true
			s.needSpace = false
			s.multiLine = true
		} else {
			if s.needNewline {
				s.buf.Write(detailSep)
				s.needNewline = false
				s.needSpace = false
			}
			if s.needSpace {
				s.buf.WriteByte(' ')
				s.needSpace = false
			}
		}
	}
	s.buf.Write(b[k:])
	return len(b), nil
}

// printer wraps a state to implement an xerrors.Printer.
type printer state

func (s *printer) Detail() bool {
	s.needNewline = true
	return true
}

func (s *printer) Print(args ...interface{}) {
	s.enhanceArgs(args)
	fmt.Fprint((*state)(s), args...)
}

func (s *printer) Printf(format string, args ...interface{}) {
	s.enhanceArgs(args)
	fmt.Fprintf((*state)(s), format, args...)
}

func (s *printer) enhanceArgs(args []interface{}) {
	prevStack := s.lastStack
	lastSeen := prevStack
	for i := range args {
		if st, ok := args[i].(pkgErr.StackTrace); ok {
			args[i], _ = ElideSharedStackTraceSuffix(prevStack, st)
			lastSeen = st
		}
		if err, ok := args[i].(error); ok {
			args[i] = &errorFormatter{err}
		}
	}
	s.lastStack = lastSeen
}

type errorFormatter struct{ err error }

// Format implements the fmt.Formatter interface.
func (ef *errorFormatter) Format(s fmt.State, verb rune) { FormatError(ef.err, s, verb) }

// ElideSharedStackTraceSuffix removes the suffix of newStack that's already
// present in prevStack. The function returns true if some entries
// were elided.
func ElideSharedStackTraceSuffix(prevStack, newStack StackTrace) (StackTrace, bool) {
	if len(prevStack) == 0 {
		return newStack, false
	}
	if len(newStack) == 0 {
		return newStack, false
	}

	// Skip over the common suffix.
	var i, j int
	for i, j = len(newStack)-1, len(prevStack)-1; i > 0 && j > 0; i, j = i-1, j-1 {
		if newStack[i] != prevStack[j] {
			break
		}
	}
	if i == 0 {
		// Keep at least one entry.
		i = 1
	}
	return newStack[:i], i < len(newStack)-1
}

// StackTrace is the type of the data for a call stack.
// This mirrors the type of the same name in github.com/pkg/errors.
type StackTrace = pkgErr.StackTrace

// StackFrame is the type of a single call frame entry.
// This mirrors the type of the same name in github.com/pkg/errors.
type StackFrame = pkgErr.Frame

// StackTraceProvider is a provider of StackTraces.
// This is, intendedly, defined to be implemented by pkg/errors.stack.
type StackTraceProvider interface {
	StackTrace() StackTrace
}
