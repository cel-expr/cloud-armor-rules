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

package cloudarmor

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// TestSuite represents a set of tests for a Cloud Armor rule expression.
type TestSuite struct {
	Name  string      `yaml:"name"`
	Expr  string      `yaml:"expr"`
	Tests []*TestCase `yaml:"tests"`
}

// TestCase represents a single test case for a Cloud Armor rule expression.
type TestCase struct {
	Name         string     `yaml:"name"`
	When         *Variables `yaml:"when"`
	ExpectOutput bool       `yaml:"expect"`
	ExpectError  string     `yaml:"error"`
}

// TestStatus represents the result of a single test case.
type TestStatus struct {
	Name string
	Pass bool
	Fail string
}

// SafeTestCase ensures that all of the variables are initialized to their default values.
func SafeTestCase(t *TestCase) *TestCase {
	if t.When == nil {
		t.When = &Variables{}
	}
	t.When = SafeVariables(t.When)
	return t
}

// TestSuiteFromYAML converts a YAML representation of a test suite to a TestSuite type.
//
// The YAML representation is expected to be a map of test suite name to a list of test cases.
//
// The return value is the TestSuite type or an error if the YAML is invalid.
func TestSuiteFromYAML(yamlBytes []byte) (*TestSuite, error) {
	ts := &TestSuite{}
	if err := yaml.Unmarshal(yamlBytes, &ts); err != nil {
		return nil, err
	}
	for i, t := range ts.Tests {
		if t.ExpectOutput && t.ExpectError != "" {
			return nil, fmt.Errorf("test case %q has both expect and error", t.Name)
		}
		ts.Tests[i] = SafeTestCase(t)
	}
	return ts, nil
}
