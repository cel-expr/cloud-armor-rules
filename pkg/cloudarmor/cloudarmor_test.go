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

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

var tests = []struct {
	name    string
	expr    string
	vars    *cloudarmor.Variables
	want    ref.Val
	version uint32
}{
	{
		name: "equality success",
		expr: "request.method == 'GET'",
		vars: &cloudarmor.Variables{
			Request: &cloudarmor.Request{
				Method: "GET",
			},
		},
		want: types.True,
	},
	{
		name: "equality failure",
		expr: "request.method == 'POST'",
		vars: &cloudarmor.Variables{
			Request: &cloudarmor.Request{
				Method: "GET",
			},
		},
		want: types.False,
	},
	{
		name: "inequality success",
		expr: "request.method != 'POST'",
		vars: &cloudarmor.Variables{
			Request: &cloudarmor.Request{
				Method: "GET",
			},
		},
		want: types.True,
	},
	{
		name: "has header - select",
		expr: "has(request.headers.user_agent)",
		vars: &cloudarmor.Variables{
			Request: &cloudarmor.Request{
				Headers: cloudarmor.HTTPHeaders(map[string]string{
					"user_agent": "found",
				}),
			},
		},
		want: types.True,
	},
	{
		name: "has header - index",
		expr: "has(request.headers['user-agent'])",
		vars: &cloudarmor.Variables{
			Request: &cloudarmor.Request{
				Headers: cloudarmor.HTTPHeaders(map[string]string{
					"user-agent": "found",
				}),
			},
		},
		want: types.True,
	},
	{
		name: "inIpRange success",
		expr: "inIpRange(origin.ip, '192.168.0.0/16')",
		vars: &cloudarmor.Variables{
			Origin: &cloudarmor.Origin{
				IP: "192.168.0.1",
			},
		},
		want: types.True,
	},
	{
		name: "inIpRange failure",
		expr: "inIpRange(origin.ip, '192.168.0.0/16')",
		vars: &cloudarmor.Variables{
			Origin: &cloudarmor.Origin{
				IP: "192.167.1.1",
			},
		},
		want: types.False,
	},
	{
		name: "equality success for tls ja4 fingerprint",
		expr: "origin.tls_ja4_fingerprint == '123456789123456789123456789123456789'",
		vars: &cloudarmor.Variables{
			Origin: &cloudarmor.Origin{
				TLSJA4Fingerprint: "123456789123456789123456789123456789",
			},
		},
		want: types.True,
	},
	{
		name: "lower case compare",
		expr: "request.method.lower() == 'get'",
		vars: &cloudarmor.Variables{
			Request: &cloudarmor.Request{
				Method: "GET",
			},
		},
		want: types.True,
	},
	{
		name: "lower case inequality",
		expr: "request.method.lower() != 'get'",
		vars: &cloudarmor.Variables{
			Request: &cloudarmor.Request{
				Method: "POST",
			},
		},
		want: types.True,
	},
	{
		name: "upper case compare",
		expr: "request.method.upper() == 'GET'",
		vars: &cloudarmor.Variables{
			Request: &cloudarmor.Request{
				Method: "get",
			},
		},
		want: types.True,
	},
	{
		name: "upper case inequality",
		expr: "request.method.upper() != 'GET'",
		vars: &cloudarmor.Variables{
			Request: &cloudarmor.Request{
				Method: "post",
			},
		},
		want: types.True,
	},
	{
		name: "base64 decode success",
		expr: "'YWJj'.base64Decode() == 'abc'",
		vars: &cloudarmor.Variables{},
		want: types.True,
	},
	{
		name: "base64 decode failure",
		expr: "'YWJj'.base64Decode() == 'def'",
		vars: &cloudarmor.Variables{},
		want: types.False,
	},
	{
		name: "conversion string to int",
		expr: "int('123') == 123",
		vars: &cloudarmor.Variables{},
		want: types.True,
	},
	{
		name: "bool field access",
		expr: "token.recaptcha_exemption.valid",
		vars: &cloudarmor.Variables{
			Token: &cloudarmor.Token{
				RecaptchaExemption: &cloudarmor.RecaptchaExemption{
					Valid: true,
				},
			},
		},
		want: types.True,
	},
	{
		name: "bool field negation",
		expr: "!token.recaptcha_action.valid",
		vars: &cloudarmor.Variables{
			Token: &cloudarmor.Token{
				RecaptchaAction: &cloudarmor.RecaptchaAction{
					Valid: false,
				},
			},
		},
		want: types.True,
	},
	{
		name: "headers comparison",
		expr: "request.headers['User-Agent'.lower()] == 'Mozilla/5.0'",
		vars: &cloudarmor.Variables{
			Request: &cloudarmor.Request{
				Headers: cloudarmor.HTTPHeaders(map[string]string{"User-Agent": "Mozilla/5.0"}),
			},
		},
		want: types.True,
	},
	{
		name: "query decode",
		expr: "request.query.urlDecode() == 'value=Url Encoded!'",
		vars: &cloudarmor.Variables{
			Request: &cloudarmor.Request{
				Query: `value=Url+Encoded%21`, // query
			},
		},
		want: types.True,
	},
	{
		name: "query decode microsoft iis-format",
		expr: "request.query.urlDecodeUni() == 'value=Unicode Url Encoded!'",
		vars: &cloudarmor.Variables{
			Request: &cloudarmor.Request{
				Query: `value=Unicode%20Url%u0020Encoded%u0021`, // query
			},
		},
		want: types.True,
	},
	{
		name: "query decode utf8 to unicode success",
		expr: "request.query.utf8ToUnicode() == '%u0100 %u0101 %u0102 %u0103 %u0400 %u0401 %u0100\\\\n%u0101 a\\\\x83t'",
		vars: &cloudarmor.Variables{
			Request: &cloudarmor.Request{
				Query: `Ā ā Ă ă Ѐ Ё Ā\nā a\x83t`, // query
			},
		},
		want: types.True,
	},
	{
		name: "query decode utf8 to unicode failure",
		expr: "request.query.utf8ToUnicode() == 'u0100 %u0101 %u0102 %u0103 %u0400 %u0401 %u0100\\\\n%u0101 a\\\\x83t'",
		vars: &cloudarmor.Variables{
			Request: &cloudarmor.Request{
				Query: `Ā ā Ă ă Ѐ Ё QA\nā a\x83t`, // query
			},
		},
		want: types.False,
	},
	{
		name: "startsWith success",
		expr: "request.query.startsWith('foo')",
		vars: &cloudarmor.Variables{
			Request: &cloudarmor.Request{
				Query: "foo=bar&baz=qux",
			},
		},
		want: types.True,
	},
	{
		name: "startsWith failure",
		expr: "request.query.startsWith('bar')",
		vars: &cloudarmor.Variables{
			Request: &cloudarmor.Request{
				Query: "foo=bar&baz=qux",
			},
		},
		want: types.False,
	},
	{
		name: "contains success for query",
		expr: "request.query.contains('bar')",
		vars: &cloudarmor.Variables{
			Request: &cloudarmor.Request{
				Query: "foo=bar&baz=qux",
			},
		},
		want: types.True,
	},
	{
		name: "contains failure for query",
		expr: "request.query.contains('bag')",
		vars: &cloudarmor.Variables{
			Request: &cloudarmor.Request{
				Query: "foo=bar&baz=qux",
			},
		},
		want: types.False,
	},
	{
		name: "contains success for path",
		expr: "request.path.lower().contains('bar')",
		vars: &cloudarmor.Variables{
			Request: &cloudarmor.Request{
				Path: "/foo/Bar/baz",
			},
		},
		want: types.True,
	},
	{
		name: "string body success",
		expr: "request.body.contains('bad_data')",
		vars: &cloudarmor.Variables{
			Request: &cloudarmor.Request{
				Body: "bad_data",
			},
		},
		want:    types.True,
		version: cloudarmor.VNext,
	},
	{
		name: "request.params missing key",
		expr: "has(request.params.nonexistent_key)",
		vars: &cloudarmor.Variables{
			Request: &cloudarmor.Request{
				Params: map[string]any{ // Directly use map[string]any
					"key": "value",
				},
			},
		},
		want:    types.False,
		version: cloudarmor.VNext,
	},
	{
		name: "request.params type mismatch",
		expr: "request.params.key == 123",
		vars: &cloudarmor.Variables{
			Request: &cloudarmor.Request{
				Params: map[string]any{
					"key": "value", // Should be a string, comparison with int should fail
				},
			},
		},
		want:    types.False,
		version: cloudarmor.VNext,
	},
	{
		name: "request.params key lookup",
		expr: "request.params['key'] == 'value'",
		vars: &cloudarmor.Variables{
			Request: &cloudarmor.Request{
				Params: map[string]any{
					"key": "value",
				},
			},
		},
		want:    types.True,
		version: cloudarmor.VNext,
	},
	{
		name: "request.params nested key lookup",
		expr: "request.params.key1.key2 == 'nestedvalue'",
		vars: &cloudarmor.Variables{
			Request: &cloudarmor.Request{
				Params: map[string]any{
					"key1": map[string]any{
						"key2": "nestedvalue",
					},
				},
			},
		},
		want:    types.True,
		version: cloudarmor.VNext,
	},
	{
		name: "nested params",
		expr: "request.params['key1']['key2'] != 'value'",
		vars: &cloudarmor.Variables{
			Request: &cloudarmor.Request{
				Params: map[string]any{
					"key1": map[string]any{
						"key2": "value2",
					},
				},
			},
		},
		want:    types.True,
		version: cloudarmor.VNext,
	},
	{
		name: "nested params field selection",
		expr: "request.params.key1.key2.startsWith('value')",
		vars: &cloudarmor.Variables{
			Request: &cloudarmor.Request{
				Params: map[string]any{
					"key1": map[string]any{
						"key2": "value2",
					},
				},
			},
		},
		want:    types.True,
		version: cloudarmor.VNext,
	},
	{
		name: "nested params backtick selection",
		expr: "request.params.`key-one`.`key-two`.contains('value')",
		vars: &cloudarmor.Variables{
			Request: &cloudarmor.Request{
				Params: map[string]any{
					"key-one": map[string]any{
						"key-two": "value=two",
					},
				},
			},
		},
		want:    types.True,
		version: cloudarmor.VNext,
	},
}

func TestRules(t *testing.T) {
	for _, v := range []uint32{cloudarmor.VCurrent, cloudarmor.VNext} {
		rules, err := cloudarmor.NewRules(cloudarmor.Version(v))
		if err != nil {
			t.Errorf("cloudarmor.NewRules() returned error: %v", err)
		}
		for _, tst := range tests {
			tc := tst
			if v < tc.version {
				continue
			}
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				ast, iss := rules.Compile(tc.expr)
				if iss != nil {
					t.Fatalf("rules.Compile() returned issues: %v", iss)
				}
				prg, err := rules.Program(ast)
				if err != nil {
					t.Fatalf("ruels.Program() returned error: %v", err)
				}
				res, _, err := prg.Eval(tc.vars)
				if err != nil {
					t.Errorf("prg.Eval() returned error: %v", err)
				}
				if res != tc.want {
					t.Errorf("prg.Eval() = %v, want %v", res, tc.want)
				}
			})
		}
	}
}

func TestRunTestSuite(t *testing.T) {
	tsData, err := os.ReadFile("../../test/http-tests.yaml")
	if err != nil {
		t.Fatalf("os.ReadFile() returned error: %v", err)
	}
	ts, err := cloudarmor.TestSuiteFromYAML(tsData)
	if err != nil {
		t.Errorf("cloudarmor.TestSuiteFromYAML() returned error: %v", err)
	}

	r, err := cloudarmor.NewRules()
	if err != nil {
		t.Fatalf("cloudarmor.NewRules() returned error: %v", err)
	}
	ast, iss := r.Compile(ts.Expr)
	if iss != nil {
		t.Fatalf("rules.Compile() returned issues: %v", iss)
	}
	prg, err := r.Program(ast)
	if err != nil {
		t.Fatalf("rules.Program() returned error: %v", err)
	}
	statuses := r.RunRuleValidation(prg, ts.Tests)
	if len(statuses) != len(ts.Tests) {
		t.Errorf("len(statuses) = %d, want %d", len(statuses), len(ts.Tests))
	}
	for _, s := range statuses {
		if s.Fail != "" {
			t.Errorf("FAIL %s/%s: %s", ts.Name, s.Name, s.Fail)
		}
	}
}
