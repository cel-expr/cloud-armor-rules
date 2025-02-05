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

// Package main provides a CLI for interacting with Cloud Armor rules.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/google/cel-go/cel"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	"github.com/cel-expr/cloud-armor-rules/pkg/cloudarmor"
)

const textFmtHeader = `# proto-file: github.com/google/cel-spec/proto/checked.proto
# proto-message: dev.cel.expr.CheckedExpr

`

type options struct {
	compile, test string
	outputFormat  string
}

func (o *options) registerFlags(fs *flag.FlagSet) {
	fs.StringVar(&o.test, "test", "", "file containing test suites for a rule expression")
	fs.StringVar(&o.compile, "compile", "", "CEL expression representing the Cloud Armor rule")
	fs.StringVar(&o.outputFormat, "output_format", "textproto", "output format (textproto, binarypb)")
}

func (o *options) validate() error {
	if o.compile == "" && o.test == "" {
		return fmt.Errorf("either -compile=<expression> or -test=<test_suite_file> is required")
	}
	if o.compile != "" && o.outputFormat != "textproto" && o.outputFormat != "binarypb" {
		return fmt.Errorf("unsupported -output_format=%s, must be textproto or binarypb", o.outputFormat)
	}
	return nil
}

type rules struct {
	*cloudarmor.Rules
}

func newRules() *rules {
	r, err := cloudarmor.NewRules()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create rules environment: %v\n", err)
		os.Exit(1)
	}
	return &rules{r}
}

func (r *rules) newAST(expr string) *cel.Ast {
	ast, err := r.Compile(expr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to compile expression: %v\n", err)
		os.Exit(1)
	}
	return ast
}

func (r *rules) printAST(ast *cel.Ast, outputFormat string) {
	pb, err := cel.AstToCheckedExpr(ast)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to convert ast to checked expr: %v\n", err)
		os.Exit(1)
	}
	if outputFormat == "textproto" {
		fmt.Println(textFmtHeader + prototext.Format(pb))
	} else {
		fmt.Println(proto.MarshalOptions{Deterministic: true}.Marshal(pb))
	}
}

func (r *rules) newProgram(ast *cel.Ast) cel.Program {
	prg, err := r.Program(ast)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create program: %v\n", err)
		os.Exit(1)
	}
	return prg
}

func main() {
	var opts options
	opts.registerFlags(flag.CommandLine)
	flag.Parse()
	if len(flag.Args()) != 0 {
		fmt.Fprintf(os.Stderr, "unexpected arguments: %v\n", flag.Args())
		os.Exit(1)
	}
	if err := opts.validate(); err != nil {
		fmt.Fprintf(os.Stderr, "invalid options: %v\n", err)
		os.Exit(1)
	}
	r := newRules()
	if opts.compile != "" {
		ast := r.newAST(opts.compile)
		r.printAST(ast, opts.outputFormat)
		os.Exit(0)
	}

	tsData, err := os.ReadFile(opts.test)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read test suite file: %v\n", err)
		os.Exit(1)
	}
	ts, err := cloudarmor.TestSuiteFromYAML(tsData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse test suite: %v\n", err)
		os.Exit(1)
	}
	ast := r.newAST(ts.Expr)
	prg := r.newProgram(ast)
	statuses := r.RunTestSuite(prg, ts.Tests)
	for _, s := range statuses {
		if s.Fail != "" {
			fmt.Fprintf(os.Stderr, "FAIL %s/%s: %s\n", ts.Name, s.Name, s.Fail)
		} else {
			fmt.Fprintf(os.Stderr, "PASS %s/%s\n", ts.Name, s.Name)
		}
	}
}
