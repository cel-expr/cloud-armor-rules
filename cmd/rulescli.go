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
	"io"
	"os"
	"strings"

	"github.com/google/cel-go/cel"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	"github.com/cel-expr/cloud-armor-rules/pkg/cloudarmor"
)

const textFmtHeader = `# proto-file: github.com/google/cel-spec/proto/checked.proto
# proto-message: dev.cel.expr.CheckedExpr

`

type options struct {
	expr, file, test      string
	outputFormat, version string
	verbose               bool
}

func (o *options) registerFlags(fs *flag.FlagSet) {
	fs.StringVar(&o.test, "test", "", "file containing test suites for a rule expression")
	fs.StringVar(&o.expr, "expr", "", "CEL expression representing the Cloud Armor rule")
	fs.StringVar(&o.file, "file", "", "File containing CEL expressions representing the Cloud Armor rule")
	fs.StringVar(&o.outputFormat, "output_format", "", "output format (textproto, binarypb)")
	fs.StringVar(&o.version, "version", "VCurrent", "valid versions (VCurrent, VNext)")
	fs.BoolVar(&o.verbose, "verbose", false, "Enable verbose logging")
}

func (o *options) validate() error {
	if o.expr == "" && o.file == "" && o.test == "" {
		return fmt.Errorf("either -expr=<expression> or -file=<file> or -test=<test_suite_file> is required")
	}
	if o.expr != "" && o.outputFormat != "" &&
		o.outputFormat != "textproto" && o.outputFormat != "binarypb" {
		return fmt.Errorf("unsupported -output_format=%s, must be textproto or binarypb", o.outputFormat)
	}
	return nil
}

type rules struct {
	*cloudarmor.Rules
}

func verboseLog(enabled bool, message string, args ...any) {
	if enabled {
		fmt.Printf(message+"\n", args...)
	}
}

func newRules(ver string) *rules {
	version := cloudarmor.VCurrent
	if ver == "VNext" {
		version = cloudarmor.VNext
	}

	r, err := cloudarmor.NewRules(cloudarmor.Version(version))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create rules environment: %v\n", err)
		os.Exit(1)
	}
	return &rules{r}
}

func (r *rules) processExprFile(filename string, outputFormat string, verbose bool) error {
	verboseLog(verbose, "Reading file: %s", filename)
	file, err := os.OpenFile(filename, os.O_RDONLY, 0644)
	if err != nil {
		fmt.Println("Failed to open file:", filename)
		return err
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		fmt.Println("Failed to read file content:", filename)
		return err
	}

	expressions := strings.Split(string(content), ";") // Expressions are separated by delimiter ';'

	LineNumber := 1
	for index, expr := range expressions {
		LineNumber += strings.Count(expr, "\n")
		expr = strings.TrimSpace(expr)
		if expr == "" {
			continue
		}

		verboseLog(verbose, "Processing expr at index: %d, line: %d, expr: %s", index, LineNumber, expr)

		ast, ok := r.newAST(expr)
		if !ok {
			return fmt.Errorf("failed to compile expression: %v", expr)
		}
		verboseLog(verbose, "Successfully compiled expression: %v", expr)

		r.printAST(ast, outputFormat)
	}

	return nil
}

func (r *rules) newAST(expr string) (*cel.Ast, bool) {

	// Convert bracket notation to dot notation
	if strings.Contains(expr, "request.params") {
		expr = strings.ReplaceAll(expr, "['", ".")
		expr = strings.ReplaceAll(expr, "']", "")
	}
	ast, err := r.Compile(expr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to compile expression: %v\n", err)
		return nil, false
	}
	return ast, true
}

func (r *rules) printAST(ast *cel.Ast, outputFormat string) {
	pb, err := cel.AstToCheckedExpr(ast)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to convert ast to checked expr: %v\n", err)
		os.Exit(1)
	}
	if outputFormat == "textproto" {
		fmt.Println(textFmtHeader + prototext.Format(pb))
	} else if outputFormat == "binarypb" {
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

	// Handle default expression
	args := flag.Args()
	if len(args) > 0 {
		opts.expr = args[0] // Assign default argument to `expr`
	}

	if err := opts.validate(); err != nil {
		fmt.Fprintf(os.Stderr, "invalid options: %v\n", err)
		os.Exit(1)
	}

	r := newRules(opts.version)

	if opts.expr != "" {
		ast, ok := r.newAST(opts.expr)
		if ok {
			r.printAST(ast, opts.outputFormat)
			os.Exit(0)
		} else {
			os.Exit(1)
		}
	}
	if opts.file != "" {
		err := r.processExprFile(opts.file, opts.outputFormat, opts.verbose)
		if err != nil {
			fmt.Println("Error processing file:", err)
			os.Exit(1)
		}
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
	ast, ok := r.newAST(ts.Expr)
	if !ok {
		os.Exit(1)
	}

	prg := r.newProgram(ast)
	statuses := r.RunRuleValidation(prg, ts.Tests)
	for _, s := range statuses {
		if s.Fail != "" {
			fmt.Fprintf(os.Stderr, "FAIL %s/%s: %s\n", ts.Name, s.Name, s.Fail)
		} else {
			fmt.Fprintf(os.Stderr, "PASS %s/%s\n", ts.Name, s.Name)
		}
	}
}
