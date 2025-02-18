# Cloud Armor Rules CLI

Cloud Armor Rules supports [Common Expression Language (CEL)](https://cel.dev)
expressions to configure its behavior. The CLI provides support for command-
line compilation and testing of Cloud Armor expressions in a manner which is
consistent with how the service will evaluate these rules.

## Getting Started

First, build the CLI:

```
go build -o rulescli github.com/cel-expr/cloud-armor-rules/cmd && chmod 0755 rulescli
```

This will produce a `rulescli` binary in the current directory which can be
executed using `./rulescli` to provide a basic usage message.

## Usage

The CLI provides two modes `-compile` and `-test`.

### Compile

The `-compile=<expr>` flag indicates that the expression provided following the
flag will be compiled and output into a textproto format. The `-output_format`
flag can be used with the `-compile` flag to produce either a textproto or
binary protocol buffer (binarypb) file as well.

Here's a simple example:

```
./rulescli -compile="request.method == 'GET'"
```

Will produce the following `dev.cel.expr.CheckedExpr` output:

```
# proto-file: github.com/google/cel-spec/proto/cel/expr/checked.proto
# proto-message: dev.cel.expr.CheckedExpr

reference_map:  {
  key:  2
  value:  {
    name:  "request.method"
  }
}
reference_map:  {
  key:  3
  value:  {
    overload_id:  "equals_string"
  }
}
type_map:  {
  key:  2
  value:  {
    primitive:  STRING
  }
}
type_map:  {
  key:  3
  value:  {
    primitive:  BOOL
  }
}
type_map:  {
  key:  4
  value:  {
    primitive:  STRING
  }
}
source_info:  {
  location:  "<input>"
  line_offsets:  24
  positions:  {
    key:  1
    value:  0
  }
  positions:  {
    key:  2
    value:  7
  }
  positions:  {
    key:  3
    value:  15
  }
  positions:  {
    key:  4
    value:  18
  }
}
expr:  {
  id:  3
  call_expr:  {
    function:  "_==_"
    args:  {
      id:  2
      ident_expr:  {
        name:  "request.method"
      }
    }
    args:  {
      id:  4
      const_expr:  {
        string_value:  "GET"
      }
    }
  }
}
```

To produce a binary protocol buffer, use the following option:

```
rulescli -compile="request.method == 'GET'" -output_format=binarypb
```

An invalid expression will produce a list of issues to be resolved from the
input:

```
> rulescli -compile "request.metho == 'GET'"
failed to compile expression: ERROR: <input>:1:1: undeclared reference to 'request' (in container '')
 | request.metho == 'GET'
 | ^
```

### Test

The `-test` flag may be used to provide a file path to a test suite written as
YAML which indicates a test `expr` value and a set of test cases whose format
is indicated below:

```yaml
name: "Test Suite Name"
expr: >
  <multiline-cel-expression>
tests:
  - name: "<case-name>"
    expect: <true|false>
    error: 'error substring'
    when: <variables>
```

The `tests` value is a list of `TestCase` objects which have only one of the
`expect` or `error` values set. A test which omits both of these fields will
implicitly expect an evaluation of `false`; however, it is best to explicitly
set the test expectation.

#### Variables

The `when: <variables>` field expects to receive a map of values whose structure
reflects the
[documented attributes](https://cloud.google.com/armor/docs/rules-language-reference#attributes)
in Cloud Armor. In Cloud Armor and in CEL, these attributes are flat, meaning
they do not reflect an object hierarchy and instead are treated as namespaced
values. In other words `request.method` is a type `string` field, but the
variable `request` is not defined. For convenience, the YAML supports a
structured object as input for the sake of simplicity and reducing repetition
of test code.

#### Execution

An end-to-end example of the file content might look as follows:

```yaml
name: http-tests
expr: >
  request.method == 'GET'
tests:
  - name: 'request-method-matches'
    expect: true
    when:
      request:
      method: GET
```

When you are ready to run your tests, provide a fully qualified file name or
referring to the

```
./rulescli -test $(pwd)'test/http-tests.yaml'
```

Disclaimer: This is not an official Google project
