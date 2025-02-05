// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cloudarmor_test

import (
	"os"
	"testing"

	"github.com/cel-expr/cloud-armor-rules/pkg/cloudarmor"
)

func TestTestSuiteFromYAML(t *testing.T) {
	testSuiteBytes, err := os.ReadFile("../../test/complex-tests.yaml")
	if err != nil {
		t.Fatalf("os.ReadFile() returned error: %v", err)
	}
	ts, err := cloudarmor.TestSuiteFromYAML(testSuiteBytes)
	if err != nil {
		t.Errorf("cloudarmor.TestSuiteFromYAML() returned error: %v", err)
	}
	if ts.Name != "complex-tests" {
		t.Errorf("ts.Name = %q, want %q", ts.Name, "complex-tests")
	}
	if len(ts.Tests) != 1 {
		t.Errorf("len(ts.Tests) = %d, want %d", len(ts.Tests), 1)
	}
	t.Logf("ts.Tests[0]: %+v", ts.Tests[0])
}
