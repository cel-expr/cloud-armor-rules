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
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"net"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/decls"
	"github.com/google/cel-go/common/operators"
	"github.com/google/cel-go/common/overloads"
	"github.com/google/cel-go/common/stdlib"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

// Rules represents a Cloud Armor rules environment.
type Rules struct {
	version uint32
	env     *cel.Env
}

// RulesOption is a functional operator for configuring the Cloud Armor rules environment.
type RulesOption func(*Rules) (*Rules, error)

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
	rules := &Rules{version: math.MaxUint32}
	for _, opt := range options {
		rules, err = opt(rules)
		if err != nil {
			return nil, err
		}
	}
	rules.env, err = cel.NewCustomEnv(cel.Lib(&library{version: rules.version}))
	return rules, err
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
	return r.env.Program(ast, prgOpts...)
}

// RunTestSuite runs a test suite against the an expression.
//
// The test suite is expected to contain a set of test cases which are executed in sequence.
// Each test case is expected to contain an expression to compile, the variables to bind to the
// expression, and the expected output or error.
//
// The return value is a slice of test statuses, one for each test case in the suite.
func (r *Rules) RunTestSuite(prg cel.Program, testCases []*TestCase) []TestStatus {
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

type library struct {
	version uint32
}

func (lib *library) LibraryName() string {
	return "google.cloud.armor.ext"
}

func (lib *library) CompileOptions() []cel.EnvOption {
	options := []cel.EnvOption{
		// Ensure that expressions are checked for common issues.
		cel.DefaultUTCTimeZone(true),
		cel.ExtendedValidations(),

		// Replace the standard macros with a single custom has macro.
		cel.ClearMacros(),
		cel.Macros(cel.GlobalMacro("has", 1, hasWithIndexMacroFactory)),
	}
	options = append(options, cloudArmorVariables(lib.version)...)
	options = append(options, cloudArmorFunctions(lib.version)...)
	return options
}

func (lib *library) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{
		cel.EvalOptions(cel.OptOptimize),
	}
}

func cloudArmorVariables(version uint32) []cel.EnvOption {
	return []cel.EnvOption{
		// Request attributes
		cel.Variable("request.method", cel.StringType),
		cel.Variable("request.headers", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("request.path", cel.StringType),
		cel.Variable("request.query", cel.StringType),
		cel.Variable("request.scheme", cel.StringType),
		// Origin attributes
		cel.Variable("origin.ip", cel.StringType),
		cel.Variable("origin.region_code", cel.StringType),
		cel.Variable("origin.asn", cel.IntType),
		cel.Variable("origin.user_ip", cel.StringType),
		cel.Variable("origin.tls_ja3_fingerprint", cel.StringType),
		cel.Variable("origin.tls_ja4_fingerprint", cel.StringType),
		// reCaptcha exemption attributes
		cel.Variable("token.recaptcha_exemption.valid", cel.BoolType),
		// reCaptcha action attributes
		cel.Variable("token.recaptcha_action.score", cel.DoubleType),
		cel.Variable("token.recaptcha_action.captcha_status", cel.StringType),
		cel.Variable("token.recaptcha_action.action", cel.StringType),
		cel.Variable("token.recaptcha_action.valid", cel.BoolType),
		// reCaptcha session attributes
		cel.Variable("token.recaptcha_session.score", cel.DoubleType),
		cel.Variable("token.recaptcha_session.valid", cel.BoolType),
	}
}

func cloudArmorFunctions(version uint32) []cel.EnvOption {
	permittedFunctions := map[string][]string{
		// logical operators
		operators.LogicalAnd: {},
		operators.LogicalOr:  {},
		operators.LogicalNot: {},
		// ordering
		operators.Less: {
			overloads.LessInt64,
			overloads.LessDouble,
		},
		operators.LessEquals: {
			overloads.LessEqualsInt64,
			overloads.LessEqualsDouble,
		},
		operators.Greater: {
			overloads.GreaterInt64,
			overloads.GreaterDouble,
		},
		operators.GreaterEquals: {
			overloads.GreaterEqualsInt64,
			overloads.GreaterEqualsDouble,
		},
		// set relations, indexing
		operators.In: {
			overloads.InMap,
		},
		operators.Index: {},
		// arithmetic
		operators.Add: {
			overloads.AddInt64,
			overloads.AddDouble,
			overloads.AddString,
		},
		operators.Subtract: {
			overloads.SubtractDouble,
			overloads.SubtractInt64,
		},
		operators.Multiply: {
			overloads.MultiplyDouble,
			overloads.MultiplyInt64,
		},
		// string operations
		overloads.Size: {
			overloads.SizeString,
		},
		overloads.TypeConvertInt: {
			overloads.StringToInt,
			overloads.IntToInt,
		},
		overloads.Matches:    {},
		overloads.Contains:   {},
		overloads.EndsWith:   {},
		overloads.StartsWith: {},

		// forward compatibility with macros
		operators.NotStrictlyFalse: {},
	}

	// Normally equality is type parameterized; however, we only support a subset of types.
	funcs := []cel.EnvOption{
		cel.Function(operators.Equals,
			cel.Overload(overloads.Equals+"_bool", []*cel.Type{cel.BoolType, cel.BoolType}, cel.BoolType),
			cel.Overload(overloads.Equals+"_double", []*cel.Type{cel.DoubleType, cel.DoubleType}, cel.BoolType),
			cel.Overload(overloads.Equals+"_int64", []*cel.Type{cel.IntType, cel.IntType}, cel.BoolType),
			cel.Overload(overloads.Equals+"_string", []*cel.Type{cel.StringType, cel.StringType}, cel.BoolType),
			cel.SingletonBinaryBinding(func(lhs, rhs ref.Val) ref.Val {
				return lhs.Equal(rhs)
			}),
		),
		cel.Function(operators.NotEquals,
			cel.Overload(overloads.NotEquals+"_bool", []*cel.Type{cel.BoolType, cel.BoolType}, cel.BoolType),
			cel.Overload(overloads.NotEquals+"_double", []*cel.Type{cel.DoubleType, cel.DoubleType}, cel.BoolType),
			cel.Overload(overloads.NotEquals+"_int64", []*cel.Type{cel.IntType, cel.IntType}, cel.BoolType),
			cel.Overload(overloads.NotEquals+"_string", []*cel.Type{cel.StringType, cel.StringType}, cel.BoolType),
			cel.SingletonBinaryBinding(func(lhs, rhs ref.Val) ref.Val {
				return types.Bool(lhs.Equal(rhs) != types.True)
			}),
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

	stdLibSubset := []*decls.FunctionDecl{}
	for _, fn := range stdlib.Functions() {
		overloads, found := permittedFunctions[fn.Name()]
		if !found {
			continue
		}
		if len(overloads) != 0 {
			fn = fn.Subset(cel.IncludeOverloads(overloads...))
		}
		stdLibSubset = append(stdLibSubset, fn)
	}
	return append(funcs, cel.FunctionDecls(stdLibSubset...))
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
