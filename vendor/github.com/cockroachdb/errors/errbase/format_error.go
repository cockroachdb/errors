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
	case verb == 'v' && s.Flag('+'):
		// Here we are going to format as per %+v, into p.buf.
		//
		// We need to start with the innermost (root cause) error first,
		// then the layers of wrapping from innermost to outermost, so as
		// to enable stack trace de-duplication. This requires a
		// post-order traversal. Since we have a linked list, the best we
		// can do is a recursion.
		p.formatRecursive(err, true /* isOutermost */, true /* withDetail */)

		// We now have all the data, we can render the result.
		p.formatEntries(err)

		// We're done formatting. Apply width/precision parameters.
		p.finishDisplay(verb)

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
		// Only the error message.
		//
		// Use an intermediate buffer because there may be alignment
		// instructions to obey in the final rendering or
		// quotes to add (for %q).
		//
		// Conceptually, we could just do
		//       p.buf.WriteString(err.Error())
		// However we also advertise that Error() can be implemented
		// by calling FormatError(), in which case we'd get an infinite
		// recursion. So we have no choice but to peel the data
		// and then assemble the pieces ourselves.
		p.formatRecursive(err, true /* isOutermost */, false /* withDetail */)
		p.formatSingleLineOutput()
		p.finishDisplay(verb)

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
	}
}

func (s *state) formatEntries(err error) {
	// The first entry at the top is special. We format it as follows:
	//
	//   <complete error message>
	//   (1) <details>
	s.formatSingleLineOutput()
	s.buf.WriteString("\n(1)")
	printEntry(&s.buf, s.entries[len(s.entries)-1])

	// All the entries that follow are printed as follows:
	//
	// Wraps: (N) <details>
	//
	for i, j := len(s.entries)-2, 2; i >= 0; i, j = i-1, j+1 {
		fmt.Fprintf(&s.buf, "\nWraps: (%d)", j)
		entry := s.entries[i]
		printEntry(&s.buf, entry)
	}

	// At the end, we link all the (N) references to the Go type of the
	// error.
	s.buf.WriteString("\nError types:")
	for i, j := len(s.entries)-1, 1; i >= 0; i, j = i-1, j+1 {
		fmt.Fprintf(&s.buf, " (%d) %T", j, s.entries[i].err)
	}
}

func printEntry(buf *bytes.Buffer, entry formatEntry) {
	if len(entry.head) > 0 {
		if entry.head[0] != '\n' {
			buf.WriteByte(' ')
		}
		buf.Write(entry.head)
	}
	if len(entry.details) > 0 {
		if len(entry.head) == 0 {
			if entry.details[0] != '\n' {
				buf.WriteByte(' ')
			}
		}
		buf.Write(entry.details)
	}
}

// formatSingleLineOutput prints the details extracted via
// formatRecursive() through the chain of errors as if .Error() has
// been called: it only prints the non-detail parts and prints them on
// one line with ": " separators.
//
// This function is used both when FormatError() is called indirectly
// from .Error(), e.g. in:
//      (e *myType) Error() { return fmt.Sprintf("%v", e) }
//      (e *myType) Format(s fmt.State, verb rune) { errors.FormatError(s, verb, e) }
//
// and also to print the first line in the output of a %+v format.
func (s *state) formatSingleLineOutput() {
	for i := len(s.entries) - 1; i >= 0; i-- {
		entry := &s.entries[i]
		if entry.elideShort {
			continue
		}
		if s.buf.Len() > 0 && len(entry.head) > 0 {
			s.buf.WriteString(": ")
		}
		s.buf.Write(entry.head)
	}
}

// formatRecursive performs a post-order traversal on the chain of
// errors to collect error details from innermost to outermost.
//
// It populates s.entries as a result.
func (s *state) formatRecursive(err error, isOutermost, withDetail bool) {
	cause := UnwrapOnce(err)
	if cause != nil {
		// Recurse first.
		s.formatRecursive(cause, false /*isOutermost*/, withDetail)
	}

	// Reinitialize the state for this stage of wrapping.
	s.wantDetail = withDetail
	s.needSpace = false
	s.needNewline = 0
	s.multiLine = false
	s.notEmpty = false
	s.hasDetail = false
	s.headBuf = nil

	seenTrace := false

	switch v := err.(type) {
	case Formatter:
		desiredShortening := v.FormatError((*printer)(s))
		if desiredShortening == nil {
			// The error wants to elide the short messages. Do it.
			for i := range s.entries {
				s.entries[i].elideShort = true
			}
		}
	case fmt.Formatter:
		// We can only use a fmt.Formatter when both the following
		// conditions are true:
		// - when it is the leaf error, because a fmt.Formatter
		//   on a wrapper also recurses.
		// - when it is not the outermost wrapper, because
		//   the Format() method is likely to be calling FormatError()
		//   to do its job and we want to avoid an infinite recursion.
		if !isOutermost && cause == nil {
			v.Format(s, 'v')
			if st, ok := err.(StackTraceProvider); ok {
				// This is likely a leaf error from github/pkg/errors.
				// The thing probably printed its stack trace on its own.
				seenTrace = true
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

	// If there's an embedded stack trace, print it.
	// This will get either a stack from pkg/errors, or ours.
	if !seenTrace {
		if st, ok := err.(StackTraceProvider); ok {
			newStack, elided := ElideSharedStackTraceSuffix(s.lastStack, st.StackTrace())
			s.lastStack = newStack
			if s.wantDetail {
				s.switchOver()
				if s.multiLine {
					s.Write([]byte("\n-- stack trace:"))
				}
				newStack.Format(s, 'v')
				if elided {
					s.Write([]byte("\n[...repeated from below...]"))
				}
			}
		}
	}

	// Collect the result.
	entry := s.collectEntry(err)
	s.entries = append(s.entries, entry)
	s.buf = bytes.Buffer{}
}

func (s *state) collectEntry(err error) formatEntry {
	entry := formatEntry{err: err}
	if s.wantDetail {
		// The buffer has been populated as a result of formatting with
		// %+v. In that case, if the printer has separated detail
		// from non-detail, we can use the split.
		if s.hasDetail {
			entry.head = s.headBuf
			entry.details = s.buf.Bytes()
		} else {
			entry.head = s.buf.Bytes()
		}
	} else {
		entry.head = s.headBuf
		if len(entry.head) > 0 && entry.head[len(entry.head)-1] != '\n' &&
			s.buf.Len() > 0 && s.buf.Bytes()[0] != '\n' {
			entry.head = append(entry.head, '\n')
		}
		entry.head = append(entry.head, s.buf.Bytes()...)
	}
	return entry
}

// formatSimple performs a best effort at extracting the details at a
// given level of wrapping when the error object does not implement
// the Formatter interface.
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

var detailSep = []byte("\n  | ")

// state tracks error printing state. It implements fmt.State.
type state struct {
	fmt.State

	// buf collects the details of the current error object
	// at a given stage of recursion in formatRecursive().
	// At each stage of recursion (level of wrapping), buf
	// contains successively:
	//
	// - at the beginning, the "simple" part of the error message --
	//   either the pre-Detail() string if the error implements Formatter,
	//   or the result of Error().
	//
	// - after the first call to Detail(), buf is copied to headBuf,
	//   then reset, then starts collecting the "advanced" part of the
	//   error message.
	buf bytes.Buffer
	// When an error's FormatError() calls Detail(), the current
	// value of buf above is copied to headBuf, and a new
	// buf is initialized.
	headBuf []byte
	// entries collects the result of formatRecursive().
	entries []formatEntry

	// hasDetail becomes true at each level of th formatRecursive()
	// recursion after the first call to .Detail(). It is used to
	// determine how to translate buf/headBuf into a formatEntry.
	hasDetail bool

	// wantDetail is set to true when the error is formatted via %+v.
	// When false, printer.Detail() will always return false and the
	// error's .FormatError() method can perform less work. (This is an
	// optimization for the common case when an error's .Error() method
	// delegates its work to its .FormatError() via fmt.Format and
	// errors.FormatError().)
	wantDetail bool

	// lastStack tracks the last stack trace observed when looking at
	// the errors from innermost to outermost. This is used to elide
	// redundant stack trace entries.
	lastStack StackTrace

	// notEmpty tracks, at each level of recursion of formatRecursive(),
	// whether there were any details printed by an error's
	// .FormatError() method. It is used to properly determine whether
	// the printout should start with a newline and padding.
	notEmpty bool
	// needSpace tracks whether the next character displayed should pad
	// using a space character.
	needSpace bool
	// needNewline tracks whether the next character displayed should
	// pad using a newline and indentation.
	needNewline int
	// multiLine tracks whether the details so far contain multiple
	// lines. It is used to determine whether an enclosed stack trace,
	// if any, should be introduced with a separator.
	multiLine bool
}

// formatEntry collects the textual details about one level of
// wrapping or the leaf error in an error chain.
type formatEntry struct {
	err error
	// head is the part of the text that is suitable for printing in the
	// one-liner summary, or when producing the output of .Error().
	head []byte
	// details is the part of the text produced in the advanced output
	// included for `%+v` formats.
	details []byte
	// elideShort, if true, elides the value of 'head' from concatenated
	// "short" messages produced by formatSingleLineOutput().
	elideShort bool
}

// String is used for debugging only.
func (e formatEntry) String() string {
	return fmt.Sprintf("formatEntry{%T, %q, %q}", e.err, e.head, e.details)
}

// Write implements io.Writer.
func (s *state) Write(b []byte) (n int, err error) {
	if len(b) == 0 {
		return 0, nil
	}
	k := 0

	sep := detailSep
	if !s.wantDetail {
		sep = []byte("\n")
	}

	for i, c := range b {
		if c == '\n' {
			// Flush all the bytes seen so far.
			s.buf.Write(b[k:i])
			// Don't print the newline itself; instead, prepare the state so
			// that the _next_ character encountered will pad with a newline.
			// This algorithm avoids terminating error details with excess
			// newline characters.
			k = i + 1
			s.needNewline++
			s.needSpace = false
			s.multiLine = true
			if s.wantDetail {
				s.switchOver()
			}
		} else {
			if s.needNewline > 0 && s.notEmpty {
				// If newline chars were pending, display them now.
				// The s.notEmpty condition ensures that we don't
				// start a detail string with excess newline characters.
				for i := 0; i < s.needNewline-1; i++ {
					s.buf.Write(detailSep[:len(sep)-1])
				}
				s.buf.Write(sep)
				s.needNewline = 0
				s.needSpace = false
			} else if s.needSpace {
				s.buf.WriteByte(' ')
				s.needSpace = false
			}
			s.notEmpty = true
		}
	}
	s.buf.Write(b[k:])
	return len(b), nil
}

// printer wraps a state to implement an xerrors.Printer.
type printer state

func (p *state) detail() bool {
	if !p.wantDetail {
		return false
	}
	if p.notEmpty {
		p.needNewline = 1
	}
	p.switchOver()
	return true
}

func (p *state) switchOver() {
	if p.hasDetail {
		return
	}
	p.headBuf = p.buf.Bytes()
	p.buf = bytes.Buffer{}
	p.notEmpty = false
	p.hasDetail = true
}

func (s *printer) Detail() bool {
	return ((*state)(s)).detail()
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
