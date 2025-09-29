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

// Package cloudarmor provides a CEL environment for local validation and evaluation of Cloud Armor
// expressions.
package cloudarmor

import (
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/env"
	"github.com/google/cel-go/common/operators"
	"github.com/google/cel-go/common/overloads"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"google.golang.org/protobuf/encoding/prototext"
	"gopkg.in/yaml.v3"

	pb "github.com/cel-expr/cloud-armor-rules/pkg/cloudarmor/proto"
)

const (
	// VCurrent supports the attributes currently available in Cloud Armor
	VCurrent uint32 = 1
	// VNext supports the next set of variables and functions to be enabled in Cloud Armor
	VNext uint32 = 2
)

//go:embed config/cloud-armor-v1.yaml
var cloudArmorV1 string

//go:embed config/cloud-armor-v2.yaml
var cloudArmorV2 string

// Rules represents a Cloud Armor rules environment.
type Rules struct {
	version uint32
	env     *cel.Env
}

// RulesOption is a functional operator for configuring the Cloud Armor rules environment.
type RulesOption func(*Rules) (*Rules, error)

// Version sets the version of the Cloud Armor rules environment.
func Version(version uint32) RulesOption {
	return func(r *Rules) (*Rules, error) {
		r.version = version
		return r, nil
	}
}

// NewRules creates a new CloudArmorRules instance.
//
// The options are used to configure the environment and the library version.
// As new functionality is added, the library version must be incremented.
//
// The standard flow of execution is to Compile() and ast and prepare it for
// execution by converting the AST to a Program(). The program can then be
// invoked against a series of inputs using program.Eval(vars).
//
// Compiled cel.Ast values can be serialized and restored to a CEL program
// by using the cel.AstToCheckedExpr() and cel.CheckedExprToAst() functions.
// Program instances are concurrency-safe and can be cached.
func NewRules(options ...RulesOption) (*Rules, error) {
	var err error
	rules := &Rules{version: VCurrent}
	for _, opt := range options {
		rules, err = opt(rules)
		if err != nil {
			return nil, err
		}
	}
	rules.env, err = cel.NewCustomEnv(
		compileOptions(rules.version)...,
	)
	return rules, err
}

// Env returns the cel.Env object for the Rules object.
func (r *Rules) Env() *cel.Env {
	return r.env
}

// Compile compiles the given expression into a cel.Ast or returns a set of issues.
func (r *Rules) Compile(expr string) (*cel.Ast, error) {
	ast, iss := r.env.Compile(expr)
	if iss != nil {
		return nil, iss.Err()
	}
	if ast.OutputType() != cel.BoolType {
		return nil, errors.New("expression must evaluate to a boolean value")
	}
	return ast, nil
}

// Program creates a new program from the given cel.Ast and accepts an optional set of CEL program
// options which can be used to alter how the expression evaluates to capture information like
// intermediate evaluation results.
func (r *Rules) Program(ast *cel.Ast, prgOpts ...cel.ProgramOption) (cel.Program, error) {
	opts := append([]cel.ProgramOption{cel.EvalOptions(cel.OptOptimize)}, prgOpts...)
	return r.env.Program(ast, opts...)
}

// RunRuleValidation runs a test suite against the an expression.
//
// The test suite is expected to contain a set of test cases which are executed in sequence.
// Each test case is expected to contain an expression to compile, the variables to bind to the
// expression, and the expected output or error.
//
// The return value is a slice of test statuses, one for each test case in the suite.
func (r *Rules) RunRuleValidation(prg cel.Program, testCases []*TestCase) []TestStatus {
	var statuses []TestStatus
	for _, tc := range testCases {
		out, _, err := prg.Eval(tc.When)
		if err != nil {
			if tc.ExpectError != "" {
				if !strings.Contains(err.Error(), tc.ExpectError) {
					statuses = append(statuses, TestStatus{
						Name: tc.Name,
						Fail: fmt.Sprintf("got error %q, wanted error containing %q", err.Error(), tc.ExpectError),
					})
				} else {
					statuses = append(statuses, TestStatus{Name: tc.Name, Pass: true})
				}
			} else {
				statuses = append(statuses, TestStatus{Name: tc.Name, Fail: err.Error()})
			}
			continue
		}
		if out == types.Bool(tc.ExpectOutput) {
			statuses = append(statuses, TestStatus{Name: tc.Name, Pass: true})
			continue
		}
		statuses = append(statuses, TestStatus{
			Name: tc.Name,
			Fail: fmt.Sprintf("expected result %v, got %v", tc.ExpectOutput, out),
		})
	}
	return statuses
}

func compileOptions(version uint32) []cel.EnvOption {
	options := []cel.EnvOption{
		// Replace the standard macros with a single custom has macro.
		cel.ClearMacros(),
		cel.Macros(cel.GlobalMacro("has", 1, hasWithIndexMacroFactory)),

		// Load the environment configuration
		func(e *cel.Env) (*cel.Env, error) {
			cloudArmorVersion := "cloud-armor-v1"
			cloudArmorConfig := cloudArmorV1
			switch version {
			case 1:
				break
			case 2:
				cloudArmorConfig = cloudArmorV2
			default:
				return nil, fmt.Errorf("unsupported cloud armor version: v%d", version)
			}
			c := env.NewConfig(cloudArmorVersion)
			if err := yaml.Unmarshal([]byte(cloudArmorConfig), c); err != nil {
				return nil, err
			}
			return cel.FromConfig(c)(e)
		},
	}
	options = append(options, cloudArmorFunctions(version)...)
	return options
}

func cloudArmorFunctions(_ uint32) []cel.EnvOption {
	// Normally equality is type parameterized; however, we only support a subset of types.
	funcs := []cel.EnvOption{
		cel.Function(operators.Equals,
			cel.Overload(overloads.Equals+"_bool", []*cel.Type{cel.BoolType, cel.BoolType}, cel.BoolType),
			cel.Overload(overloads.Equals+"_double", []*cel.Type{cel.DoubleType, cel.DoubleType}, cel.BoolType),
			cel.Overload(overloads.Equals+"_int64", []*cel.Type{cel.IntType, cel.IntType}, cel.BoolType),
			cel.Overload(overloads.Equals+"_string", []*cel.Type{cel.StringType, cel.StringType}, cel.BoolType),
		),
		cel.Function(operators.NotEquals,
			cel.Overload(overloads.NotEquals+"_bool", []*cel.Type{cel.BoolType, cel.BoolType}, cel.BoolType),
			cel.Overload(overloads.NotEquals+"_double", []*cel.Type{cel.DoubleType, cel.DoubleType}, cel.BoolType),
			cel.Overload(overloads.NotEquals+"_int64", []*cel.Type{cel.IntType, cel.IntType}, cel.BoolType),
			cel.Overload(overloads.NotEquals+"_string", []*cel.Type{cel.StringType, cel.StringType}, cel.BoolType),
		),
		cel.Function("inIpRange", cel.Overload("inIpRange_string", []*cel.Type{cel.StringType, cel.StringType}, cel.BoolType,
			cel.BinaryBinding(func(ip, ipRange ref.Val) ref.Val {
				ipStr := string(ip.(types.String))
				ipRangeStr := string(ipRange.(types.String))
				netIP := net.ParseIP(ipStr)
				if netIP == nil {
					return types.NewErr("invalid IP address: %s", ipStr)
				}
				_, netIPRange, err := net.ParseCIDR(ipRangeStr)
				if err != nil {
					return types.NewErr("invalid IP range: %s", ipRangeStr)
				}
				return types.Bool(netIPRange.Contains(netIP))
			}))),
		cel.Function("lower", cel.MemberOverload("string_lower", []*cel.Type{cel.StringType}, cel.StringType,
			cel.UnaryBinding(func(str ref.Val) ref.Val {
				s := string(str.(types.String))
				return lowerASCII(s)
			}))),
		cel.Function("upper", cel.MemberOverload("string_upper", []*cel.Type{cel.StringType}, cel.StringType,
			cel.UnaryBinding(func(str ref.Val) ref.Val {
				s := string(str.(types.String))
				return upperASCII(s)
			}))),
		cel.Function("base64Decode", cel.MemberOverload("base64Decode_string", []*cel.Type{cel.StringType}, cel.StringType,
			cel.UnaryBinding(func(str ref.Val) ref.Val {
				s := string(str.(types.String))
				// C++ version does these character replacements, but it's not clear why.
				// strings::ReplaceCharacters(&base64_encoded_input, "-", '+');
				// strings::ReplaceCharacters(&base64_encoded_input, "_", '/');
				return base64DecodeString(s)
			}))),
		cel.Function("urlDecode", cel.MemberOverload("urlDecode_string", []*cel.Type{cel.StringType}, cel.StringType,
			cel.UnaryBinding(func(urlStr ref.Val) ref.Val {
				s := string(urlStr.(types.String))
				return urlDecodeString(s)
			}))),
		cel.Function("urlDecodeUni", cel.MemberOverload("urlDecodeUni_string", []*cel.Type{cel.StringType}, cel.StringType,
			cel.UnaryBinding(func(urlStr ref.Val) ref.Val {
				s := string(urlStr.(types.String))
				return urlDecodeUniString(s)
			}))),
		cel.Function("utf8ToUnicode", cel.MemberOverload("utf8ToUnicode_string", []*cel.Type{cel.StringType}, cel.StringType,
			cel.UnaryBinding(func(urlStr ref.Val) ref.Val {
				s := string(urlStr.(types.String))
				return utf8ToUnicodeString(s)
			}))),
	}
	return funcs
}

func hasWithIndexMacroFactory(mef cel.MacroExprFactory, target ast.Expr, args []ast.Expr) (ast.Expr, *cel.Error) {
	arg := args[0]
	// The has() macro with field selection, as supported by CEL: has(msg.field)
	if arg.Kind() == ast.SelectKind {
		sel := arg.AsSelect()
		return mef.NewPresenceTest(sel.Operand(), sel.FieldName()), nil
	}
	// The has() macro for index operations, as a workaround for Cloud Armor: has(msg['field'])
	if arg.Kind() != ast.CallKind {
		return nil, nil
	}
	call := arg.AsCall()
	if call.FunctionName() != operators.Index {
		return nil, nil
	}
	if call.IsMemberFunction() || len(call.Args()) != 2 {
		return nil, nil
	}
	obj := call.Args()[0]
	field := call.Args()[1]
	if field.Kind() != ast.LiteralKind || field.AsLiteral().Type() != cel.StringType {
		return nil, nil
	}
	return mef.NewPresenceTest(obj, field.AsLiteral().Value().(string)), nil
}

func lowerASCII(str string) ref.Val {
	runes := []rune(str)
	for i, r := range str {
		if r <= unicode.MaxASCII {
			r = unicode.ToLower(r)
			runes[i] = r
		}
	}
	return types.String(runes)
}

func upperASCII(str string) ref.Val {
	runes := []rune(str)
	for i, r := range runes {
		if r <= unicode.MaxASCII {
			r = unicode.ToUpper(r)
			runes[i] = r
		}
	}
	return types.String(runes)
}

func base64DecodeString(str string) ref.Val {
	b, err := base64.StdEncoding.DecodeString(str)
	if err == nil {
		return types.String(b)
	}
	_, tryAltEncoding := err.(base64.CorruptInputError)
	if !tryAltEncoding {
		return types.NewErrFromString(err.Error())
	}
	b, err = base64.RawStdEncoding.DecodeString(str)
	if err != nil {
		return types.NewErrFromString(err.Error())
	}
	return types.String(b)
}

func urlDecodeString(str string) ref.Val {
	// Golang's implementation is more strict than the Cloud Armor one
	// as it will error if %XX cannot be unescaped. This should be fine
	// for open sourcing.
	// Possibly use the more error tolerant version of net/url#Url.Query()
	res, err := url.QueryUnescape(str)
	if err != nil {
		return types.NewErrFromString(err.Error())
	}
	return types.String(res)
}

func urlDecodeUniString(str string) ref.Val {
	var sb strings.Builder
	strLen := len(str)
	for idx := 0; idx < strLen; idx++ {
		c := str[idx]
		if c == '%' {
			if idx+1 >= len(str) {
				return types.NewErrFromString("invalid URL escape sequence")
			}
			c1 := str[idx+1]
			if c1 == 'u' || c1 == 'U' {
				if idx+5 >= len(str) {
					return types.NewErrFromString("invalid URL escape sequence")
				}
				r, err := strconv.Unquote("\"\\u" + str[idx+2:idx+6] + "\"")
				if err == nil {
					sb.WriteString(r)
					idx += 5
					continue
				}
			}
			if idx+2 >= len(str) {
				return types.NewErrFromString("invalid URL escape sequence")
			}
			res, err := url.QueryUnescape(str[idx : idx+3])
			if err != nil {
				sb.WriteByte(c)
			} else {
				sb.WriteString(res)
				idx += 2
			}
		} else if c == '+' {
			sb.WriteByte(' ')
		} else {
			sb.WriteByte(c)
		}
	}
	return types.String(sb.String())
}

func writeHexStr(sb *strings.Builder, v int) {
	kHexChar := []rune{'0', '1', '2', '3', '4', '5', '6', '7',
		'8', '9', 'a', 'b', 'c', 'd', 'e', 'f'}

	sb.WriteString("%u")

	if v > 0xfffff {
		sb.WriteRune(kHexChar[v>>20])
	}
	if v > 0xffff {
		sb.WriteRune(kHexChar[(v>>16)&0xf])
	}
	sb.WriteRune(kHexChar[(v>>12)&0xf])
	sb.WriteRune(kHexChar[(v>>8)&0xf])
	sb.WriteRune(kHexChar[(v>>4)&0xf])
	sb.WriteRune(kHexChar[v&0xf])
}

func utf8ToUnicodeString(str string) ref.Val {
	var sb strings.Builder

	for _, r := range str {
		if r < 0x80 {
			// ASCII character, write directly
			sb.WriteRune(r)
		} else if r < 0x800 {
			// UTF-8 two-byte sequence
			writeHexStr(&sb, int(r))
		} else if r < 0x10000 {
			// UTF-8 three-byte sequence (excluding surrogate pairs)
			if r >= 0xD800 && r <= 0xDFFF {
				sb.WriteRune(r) // Copy invalid range directly
			} else {
				writeHexStr(&sb, int(r))
			}
		} else if r < 0x110000 {
			// UTF-8 four-byte sequence
			writeHexStr(&sb, int(r))
		} else {
			// Invalid Unicode, copy as-is
			sb.WriteRune(r)
		}
	}
	return types.String(sb.String())
}

func ParseVendorRuleset(content []byte) error {

	var rulesetCollection pb.VendorRulesetCollection

	// Unmarshal the text-formatted content into the struct.
	err := prototext.Unmarshal(content, &rulesetCollection)
	if err != nil {
		return fmt.Errorf("failed to unmarshal VendorRulesetCollection: %w", err)
	}

	return nil
}
