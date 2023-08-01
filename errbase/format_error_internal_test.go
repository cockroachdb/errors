// Copyright 2020 The Cockroach Authors.
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

package errbase

import (
	goErr "errors"
	"fmt"
	"testing"

	"github.com/cockroachdb/redact"
)

type wrapMini struct {
	msg   string
	cause error
}

func (e *wrapMini) Error() string {
	return e.msg
}

func (e *wrapMini) Unwrap() error {
	return e.cause
}

func TestFormatErrorInternal(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		fmtStr   string
		expected string
	}{
		{
			name:     "single wrapper",
			err:      fmt.Errorf("%w", fmt.Errorf("a%w", goErr.New("b"))),
			fmtStr:   "%s",
			expected: "ab",
		},
		{
			name:     "simple multi-wrapper",
			err:      goErr.Join(goErr.New("a"), goErr.New("b")),
			fmtStr:   "%s",
			expected: "a\nb",
		},
		{
			name:     "simple multi-wrapper with single-cause chains inside",
			err:      goErr.Join(fmt.Errorf("a%w", goErr.New("b")), fmt.Errorf("c%w", goErr.New("d"))),
			fmtStr:   "%s",
			expected: "ab\ncd",
		},
		{
			name:   "simple multi-wrapper with single-cause chains inside (verbose)",
			err:    goErr.Join(fmt.Errorf("a%w", goErr.New("b")), fmt.Errorf("c%w", goErr.New("d"))),
			fmtStr: "%+v",
			expected: `ab
(1) ab
  | cd
Wraps: (2) cd
| Wraps: (3) d
Wraps: (4) ab
| Wraps: (5) b
Error types: (1) *errors.joinError (2) *fmt.wrapError (3) *errors.errorString (4) *fmt.wrapError (5) *errors.errorString`,
		},
		{
			name:     "test wrapMini",
			err:      &wrapMini{"whoa: d", goErr.New("d")},
			fmtStr:   "%s",
			expected: "whoa: d",
		},
		{
			name:     "multi-wrapper where not all children should be elided during printing",
			err:      fmt.Errorf(""),
			fmtStr:   "%s",
			expected: "whoa: d\nwhoa: d zz",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fe := Formattable(tt.err)
			s := fmt.Sprintf(tt.fmtStr, fe)
			if s != tt.expected {
				t.Errorf("\nexpected: \n%s\nbut got:\n%s\n", tt.expected, s)
			}
		})
	}
}

func TestPrintEntry(t *testing.T) {
	b := func(s string) []byte { return []byte(s) }

	testCases := []struct {
		entry formatEntry
		exp   string
	}{
		{formatEntry{}, ""},
		{formatEntry{head: b("abc")}, " abc"},
		{formatEntry{head: b("abc\nxyz")}, " abc\nxyz"},
		{formatEntry{details: b("def")}, " def"},
		{formatEntry{details: b("def\nxyz")}, " def\nxyz"},
		{formatEntry{head: b("abc"), details: b("def")}, " abcdef"},
		{formatEntry{head: b("abc\nxyz"), details: b("def")}, " abc\nxyzdef"},
		{formatEntry{head: b("abc"), details: b("def\n  | xyz")}, " abcdef\n  | xyz"},
		{formatEntry{head: b("abc\nxyz"), details: b("def\n  | xyz")}, " abc\nxyzdef\n  | xyz"},
	}

	for _, tc := range testCases {
		s := state{}
		s.printEntry(tc.entry)
		if s.finalBuf.String() != tc.exp {
			t.Errorf("%s: expected %q, got %q", tc.entry, tc.exp, s.finalBuf.String())
		}
	}
}

func TestFormatSingleLineOutput(t *testing.T) {
	b := func(s string) []byte { return []byte(s) }
	testCases := []struct {
		entries []formatEntry
		exp     string
	}{
		{[]formatEntry{{}}, ``},
		{[]formatEntry{{head: b(`a`)}}, `a`},
		{[]formatEntry{{head: b(`a`)}, {head: b(`b`)}, {head: b(`c`)}}, `c: b: a`},
		{[]formatEntry{{}, {head: b(`b`)}}, `b`},
		{[]formatEntry{{head: b(`a`)}, {}}, `a`},
		{[]formatEntry{{head: b(`a`)}, {}, {head: b(`c`)}}, `c: a`},
		{[]formatEntry{{head: b(`a`), elideShort: true}, {head: b(`b`)}}, `b`},
		{[]formatEntry{{head: b("abc\ndef")}, {head: b("ghi\nklm")}}, "ghi\nklm: abc\ndef"},
	}

	for _, tc := range testCases {
		s := state{entries: tc.entries}
		s.formatSingleLineOutput()
		if s.finalBuf.String() != tc.exp {
			t.Errorf("%s: expected %q, got %q", tc.entries, tc.exp, s.finalBuf.String())
		}
	}
}

func TestPrintEntryRedactable(t *testing.T) {
	sm := string(redact.StartMarker())
	em := string(redact.EndMarker())
	esc := string(redact.EscapeMarkers(redact.StartMarker()))
	b := func(s string) []byte { return []byte(s) }
	q := func(s string) string { return sm + s + em }

	testCases := []struct {
		entry formatEntry
		exp   string
	}{
		// If the entries are not redactable, they may contain arbitrary
		// characters; they get enclosed in redaction markers in the final output.
		{formatEntry{}, ""},
		{formatEntry{head: b("abc")}, " " + q("abc")},
		{formatEntry{head: b("abc\nxyz")}, " " + q("abc") + "\n" + q("xyz")},
		{formatEntry{details: b("def")}, " " + q("def")},
		{formatEntry{details: b("def\nxyz")}, " " + q("def") + "\n" + q("xyz")},
		{formatEntry{head: b("abc"), details: b("def")}, " " + q("abc") + q("def")},
		{formatEntry{head: b("abc\nxyz"), details: b("def")}, " " + q("abc") + "\n" + q("xyz") + q("def")},
		{formatEntry{head: b("abc"), details: b("def\n  | xyz")}, " " + q("abc") + q("def") + "\n" + q("  | xyz")},
		{formatEntry{head: b("abc\nxyz"), details: b("def\n  | xyz")}, " " + q("abc") + "\n" + q("xyz") + q("def") + "\n" + q("  | xyz")},
		// If there were markers in the entry, they get escaped in the output.
		{formatEntry{head: b("abc" + em + sm), details: b("def" + em + sm)}, " " + q("abc"+esc+esc) + q("def"+esc+esc)},

		// If the entries are redactable, then whatever characters they contain
		// are assumed safe and copied as-is to the final output.
		{formatEntry{redactable: true}, ""},
		{formatEntry{redactable: true, head: b("abc")}, " abc"},
		{formatEntry{redactable: true, head: b("abc\nxyz")}, " abc\nxyz"},
		{formatEntry{redactable: true, details: b("def")}, " def"},
		{formatEntry{redactable: true, details: b("def\nxyz")}, " def\nxyz"},
		{formatEntry{redactable: true, head: b("abc"), details: b("def")}, " abcdef"},
		{formatEntry{redactable: true, head: b("abc\nxyz"), details: b("def")}, " abc\nxyzdef"},
		{formatEntry{redactable: true, head: b("abc"), details: b("def\n  | xyz")}, " abcdef\n  | xyz"},
		{formatEntry{redactable: true, head: b("abc\nxyz"), details: b("def\n  | xyz")}, " abc\nxyzdef\n  | xyz"},
		// Entry already contains some markers.
		{formatEntry{redactable: true, head: b("a " + q("bc")), details: b("d " + q("ef"))}, " a " + q("bc") + "d " + q("ef")},
	}

	for _, tc := range testCases {
		s := state{redactableOutput: true}
		s.printEntry(tc.entry)
		if s.finalBuf.String() != tc.exp {
			t.Errorf("%s: expected %q, got %q", tc.entry, tc.exp, s.finalBuf.String())
		}
	}
}

func TestFormatSingleLineOutputRedactable(t *testing.T) {
	sm := string(redact.StartMarker())
	em := string(redact.EndMarker())
	// 	esc := string(redact.EscapeMarkers(redact.StartMarker()))
	b := func(s string) []byte { return []byte(s) }
	q := func(s string) string { return sm + s + em }

	testCases := []struct {
		entries []formatEntry
		exp     string
	}{
		// If the entries are not redactable, then whatever characters they contain
		// get enclosed within redaction markers.
		{[]formatEntry{{}}, ``},
		{[]formatEntry{{head: b(`a`)}}, q(`a`)},
		{[]formatEntry{{head: b(`a`)}, {head: b(`b`)}, {head: b(`c`)}}, q(`c`) + ": " + q(`b`) + ": " + q(`a`)},
		{[]formatEntry{{}, {head: b(`b`)}}, q(`b`)},
		{[]formatEntry{{head: b(`a`)}, {}}, q(`a`)},
		{[]formatEntry{{head: b(`a`)}, {}, {head: b(`c`)}}, q(`c`) + ": " + q(`a`)},
		{[]formatEntry{{head: b(`a`), elideShort: true}, {head: b(`b`)}}, q(`b`)},
		{[]formatEntry{{head: b("abc\ndef")}, {head: b("ghi\nklm")}}, q("ghi") + "\n" + q("klm") + ": " + q("abc") + "\n" + q("def")},

		// If some entries are redactable but not others, then
		// only those that are redactable are passed through.
		{[]formatEntry{{redactable: true}}, ``},
		{[]formatEntry{{redactable: true, head: b(`a`)}}, `a`},
		{[]formatEntry{{redactable: true, head: b(`a`)}, {head: b(`b`)}, {redactable: true, head: b(`c`)}}, `c: ` + q(`b`) + `: a`},

		{[]formatEntry{{redactable: true}, {head: b(`b`)}}, q(`b`)},
		{[]formatEntry{{}, {redactable: true, head: b(`b`)}}, `b`},
		{[]formatEntry{{redactable: true, head: b(`a`)}, {}}, `a`},
		{[]formatEntry{{head: b(`a`)}, {redactable: true}}, q(`a`)},

		{[]formatEntry{{head: b(`a`)}, {}, {head: b(`c`)}}, q(`c`) + `: ` + q(`a`)},
		{[]formatEntry{{head: b(`a`)}, {redactable: true}, {head: b(`c`)}}, q(`c`) + `: ` + q(`a`)},
		{[]formatEntry{{head: b(`a`), elideShort: true, redactable: true}, {head: b(`b`)}}, q(`b`)},
		{[]formatEntry{{redactable: true, head: b("abc\ndef")}, {head: b("ghi\nklm")}}, q("ghi") + "\n" + q("klm") + ": abc\ndef"},
		{[]formatEntry{{head: b("abc\ndef")}, {redactable: true, head: b("ghi\nklm")}}, "ghi\nklm: " + q("abc") + "\n" + q("def")},
		// Entry already contains some markers.
		{[]formatEntry{{redactable: true, head: b(`a` + q(" b"))}, {redactable: true, head: b(`c ` + q("d"))}}, `c ` + q(`d`) + `: a` + q(` b`)},
	}

	for _, tc := range testCases {
		s := state{entries: tc.entries, redactableOutput: true}
		s.formatSingleLineOutput()
		if s.finalBuf.String() != tc.exp {
			t.Errorf("%s: expected %q, got %q", tc.entries, tc.exp, s.finalBuf.String())
		}
	}
}
