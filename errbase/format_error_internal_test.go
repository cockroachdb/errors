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
	"bytes"
	"testing"
)

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
		var buf bytes.Buffer
		printEntry(&buf, tc.entry)
		if buf.String() != tc.exp {
			t.Fatalf("%s: expected %q, got %q", tc.entry, tc.exp, buf.String())
		}
	}
}
