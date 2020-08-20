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

package errutil

import "testing"

func TestHasVerbW(t *testing.T) {
	testCase := []struct {
		format   string
		expected bool
	}{
		{"", false},
		{" abc ", false},
		{" a%%b ", false},
		{"%3d %q", false},
		{"%+-#[5]3.4d %q", false},
		{"abc %%w", false},
		{"abc %%%w", true},
		{"abc %%%+-#[5]3.4w", true},
		{"%w", true},
		{"msg: %w", true},
		{"in %w between", true},
	}

	for _, tc := range testCase {
		actual := hasVerbW.MatchString(tc.format)
		if actual != tc.expected {
			t.Errorf("%q: expected %v, got %v", tc.format, tc.expected, actual)
		}
	}
}
