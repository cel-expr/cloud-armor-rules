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

The CLI provides four modes `-expr`, `-file`, `-test` and `-textproto`.



### expr

The `-expr=<expr>` flag indicates that the expression provided following the
flag will be compiled and output into a textproto format. The `-output_format`
flag can be used with the `-expr` flag to produce either a textproto or binary
protocol buffer (binarypb) file as well.

Here's a simple example:

```
rulescli -expr="request.method == 'GET'"
```

If no flag is specified, the default behavior is equivalent to using -expr:

```
rulescli "request.method == 'GET'"
```

If used with output_format as textproto:

```
rulescli "request.method == 'GET'" -output_format=textproto
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
rulescli -expr="request.method == 'GET'" -output_format=binarypb
```

An invalid expression will produce a list of issues to be resolved from the
input:

```
> rulescli -expr "request.metho == 'GET'"
failed to compile expression: ERROR: <input>:1:1: undeclared reference to 'request' (in container '')
 | request.metho == 'GET'
 | ^
```

By default, these expressions would be able to test the currently exposed
Cloud armor attributes.
To test the next versions of attributes, like request.params and request.body,
set the version of the expressions to VNext as follow:

```
rulescli -expr="request.method == 'GET'" -version VNext
```

### file

The `-file=<filename>` flag indicates that the expressions contained in the
provided file will be compiled.

Here's a simple example:

```
rulescli -file="fileExpr.txt"
```

Expressions in the file should be separated by the delimiter ';' and could
extend to multiline expressions.

Contents for file fileExpr.txt:

```
request.method == "POST";
request.query.contains('XyZ') &&
request.path.startsWith('path');
request.path1.startsWith('path1') || request.method == "GET";
```

Expected output:

```
failed to compile expression: ERROR: <input>:1:1: undeclared reference to 'request' (in container '')
 | request.path1.startsWith('path1') || request.method == "GET"
 | ^
Error processing file: failed to compile expression: request.path1.startsWith('path1') || request.method == "GET"
```

Whereas, additional information could be fetched using -verbose flag as follow:

```
rulescli -file="test/fileExpr.txt" -verbose
```

Expected Output:

```
Reading file: fileExpr.txt

Processing expr at index:  0 , line:  1  expr:  request.method == "POST"
Successfully compiled expression: request.method == "POST"

Processing expr at index:  1 , line:  3  expr:  request.query.contains('XyZ') &&
request.path.startsWith('path')
Successfully compiled expression: request.query.contains('XyZ') &&
request.path.startsWith('path')

Processing expr at index:  2 , line:  4  expr:  request.path1.startsWith('path1') || request.method == "GET"
failed to compile expression: ERROR: <input>:1:1: undeclared reference to 'request' (in container '')
 | request.path1.startsWith('path1') || request.method == "GET"
 | ^
Error processing file: failed to compile expression: request.path1.startsWith('path1') || request.method == "GET"
```

whereas, for the following contents for file fileExpr.txt:

```
request.method == "POST" &&
request.query.contains('XyZ');
request.path.startsWith('path');;

request.path.startsWith('path1') || request.path.startsWith('path2');
request.scheme == 'http'; request.scheme == 'https';
request.headers['User-Agent'].contains('Chrome');

request.scheme == 'http' && request.method == 'GET' ||
request.path.startsWith('/path');
```

expected output:

```
Successfully compiled expression: request.method == "POST" &&
request.query.contains('XyZ')
Successfully compiled expression: request.path.startsWith('path')
Successfully compiled expression: request.path.startsWith('path1') || request.path.startsWith('path2')
Successfully compiled expression: request.scheme == 'http'
Successfully compiled expression: request.scheme == 'https'
Successfully compiled expression: request.headers['User-Agent'].contains('Chrome')
Successfully compiled expression: request.scheme == 'http' && request.method == 'GET' ||
request.path.startsWith('/path')
```

To print the AST expressions of CEL expressions from a file as textproto:

```
rulescli -file="test/fileExpr.txt" -output_format=textproto
```

By default, file flag would allow to test the currently exposed
Cloud armor attributes.
To test the next versions of attributes, like request.params and request.body,
set the version of the expressions to VNext as follow:

```
rulescli -file="test/fileExpr.txt" -version VNext
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

#### New Attributes (Proposed for NextVersion)

1.  request.body Represents the entire POST Body as string. e.g. Expression:
    request.body.contains('bad_data')
2.  request.params It represents the query_parameters from URL in GET requests
    as well as key-value parameters from POST Body.

    ```
    e.g. for request curl "https://www.example.com/nonauth/random1.cs?dest=/somepath"
    ```

    expression would be:

    ```
    has(request.params.dest) or has(request.params['dest'])
    ```

    Similarly, it also supports accessing the nested keys as below:

    ```
    request.params.keys.key1 or request.params['keys']['key1']
    ```

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

### Textproto

The `-textproto=<filename>` flag is used to validate a file containing a `VendorRulesetCollection` in the text protobuf format. The tool attempts to parse the file and will report any syntactical errors it finds. This is useful for checking the validity of a ruleset collection before it is used.

**Example Usage:**

Assuming you have a file named `my_ruleset.txt` with content in the `VendorRulesetCollection` format:

```textproto
# A sample VendorRulesetCollection
uuid: "123e4567-e89b-12d3-a456-426614174000"
ruleset_metadata: {
  owner: "Imperva"
  description: "Initial set of rules."
}
rule_sets: {
  name: "sqli-rules"
  category: "sqli"
  rules: {
    id: "191190"
    cel_expression: "request.headers['user-agent'].contains('sqlmap')"
  }
}
```

You can validate this file by running the following command. If the file is valid, the command will exit successfully. If there are syntactical errors, it will print them to the console.

```sh
./rulescli -textproto="my_ruleset.txt"
```

Disclaimer: This is not an official Google project
