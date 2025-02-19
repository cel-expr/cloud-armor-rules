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

func TestVariablesFromYAML(t *testing.T) {
	varBytes, err := os.ReadFile("../../test/variables.yaml")
	if err != nil {
		t.Fatalf("os.ReadFile() returned error: %v", err)
	}
	v, err := cloudarmor.VariablesFromYAML(varBytes)
	if err != nil {
		t.Errorf("cloudarmor.VariablesFromYAML() returned error: %v", err)
	}
	if v.Request.Method != "GET" {
		t.Errorf("v.Request.Method = %q, want %q", v.Request.Method, "GET")
	}
	if v.Request.Headers["host"] != "www.google.com" {
		t.Errorf("v.Request.Headers['host'] = %q, want %q", v.Request.Headers["host"], "www.google.com")
	}
	if v.Request.Path != "/search" {
		t.Errorf("v.Request.Path = %q, want %q", v.Request.Path, "/search")
	}
	if v.Request.Query != "q=google%21" {
		t.Errorf("v.Request.Query = %q, want %q", v.Request.Query, "q=google%21")
	}
	if v.Request.Scheme != "https" {
		t.Errorf("v.Request.Scheme = %q, want %q", v.Request.Scheme, "https")
	}
	if v.Origin.IP != "1.2.3.4" {
		t.Errorf("v.Origin.IP = %q, want %q", v.Origin.IP, "1.2.3.4")
	}
	if v.Origin.RegionCode != "US" {
		t.Errorf("v.Origin.RegionCode = %q, want %q", v.Origin.RegionCode, "US")
	}
	if v.Token.RecaptchaExemption.Valid != true {
		t.Errorf("v.Token.RecaptchaExemption.Valid = %v, want %v", v.Token.RecaptchaExemption.Valid, true)
	}
	if v.Token.RecaptchaAction.Valid != true {
		t.Errorf("v.Token.RecaptchaAction.Valid = %v, want %v", v.Token.RecaptchaAction.Valid, true)
	}
	if v.Token.RecaptchaSession.Valid != true {
		t.Errorf("v.Token.RecaptchaSession.Valid = %v, want %v", v.Token.RecaptchaSession.Valid, true)
	}
}

func TestRequestParams(t *testing.T) {
	params := map[string]any{
		"key": "value",
		"nested": map[string]any{
			"key2": "nestedvalue",
		},
	}

	v := &cloudarmor.Variables{
		Request: &cloudarmor.Request{
			Params: params,
		},
	}

	if v.Request.Params["key"] != "value" {
		t.Errorf("v.Request.Params['key'] = %v, want value", v.Request.Params["key"])
	}

	nestedParams, ok := v.Request.Params["nested"].(map[string]any)
	if !ok {
		t.Fatalf("v.Request.Params['nested'] is not a map")
	}
	if nestedParams["key2"] != "nestedvalue" {
		t.Errorf("v.Request.Params['nested']['key2'] = %v, want nestedvalue", nestedParams["key2"])
	}
}
