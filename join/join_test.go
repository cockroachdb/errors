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

package join

import (
	"errors"
	"testing"
)

func TestJoin(t *testing.T) {
	e := Join(errors.New("abc123"), errors.New("def456"))
	expected := "abc123\ndef456"
	if e.Error() != expected {
		t.Errorf("Expected: %s; Got: %s", expected, e.Error())
	}

	e = Join(errors.New("abc123"), nil, errors.New("def456"), nil)
	if e.Error() != expected {
		t.Errorf("Expected: %s; Got: %s", expected, e.Error())
	}

	e = Join(nil, nil, nil)
	if e != nil {
		t.Errorf("expected nil error")
	}
}
