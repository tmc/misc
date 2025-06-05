/*
Package main implements de-minimis-non-curat-lex, a tool that transforms Go test files
to remove github.com/stretchr/testify dependencies and replace them with standard
library testing features and the cmp package.

The name "de-minimis-non-curat-lex" is a Latin legal maxim meaning "the law does not
concern itself with trifles" - a playful reference to how this tool helps you dismiss
the trifling concerns of external test dependencies in favor of the standard library.

# Usage

	de-minimis-non-curat-lex [flags] [path ...]

The tool processes Go test files in the specified paths (defaults to current directory).
It identifies testify usage patterns and converts them to equivalent stdlib + cmp code.

# Examples

Convert all test files in current directory:

	de-minimis-non-curat-lex

Convert specific file:

	de-minimis-non-curat-lex internal/parser/parser_test.go

Convert entire package tree:

	de-minimis-non-curat-lex ./...

Dry run to see what would change:

	de-minimis-non-curat-lex -dry-run ./...

# Flags

	-dry-run
		Show what changes would be made without modifying files

	-write
		Write changes back to source files (default true)

	-diff
		Display diffs of changes

	-v
		Verbose output showing transformation details

	-preserve-messages
		Keep original assertion messages when possible

	-stdlib-only
		Use only stdlib testing.T methods, no cmp package

# Transformations

The tool performs the following conversions:

## Basic Assertions

	assert.Equal(t, expected, actual)           → if got != want { t.Errorf(...) }
	assert.Equal(t, expected, actual, "msg")    → if got != want { t.Errorf("msg: ...") }
	assert.NotEqual(t, unexpected, actual)      → if got == dontwant { t.Errorf(...) }
	assert.True(t, condition)                   → if !condition { t.Errorf(...) }
	assert.False(t, condition)                  → if condition { t.Errorf(...) }
	assert.Nil(t, value)                        → if value != nil { t.Errorf(...) }
	assert.NotNil(t, value)                     → if value == nil { t.Errorf(...) }
	assert.Empty(t, value)                      → if len(value) != 0 { t.Errorf(...) }
	assert.Len(t, value, length)                → if len(value) != length { t.Errorf(...) }

## Complex Assertions (using cmp)

	assert.Equal(t, complexStruct, actual)      → if diff := cmp.Diff(want, got); diff != "" { t.Errorf(..., diff) }
	assert.ElementsMatch(t, expected, actual)   → uses cmp with cmpopts.SortSlices
	assert.Contains(t, haystack, needle)        → uses strings.Contains or slices.Contains
	assert.Greater(t, a, b)                     → if a <= b { t.Errorf(...) }
	assert.InDelta(t, expected, actual, delta)  → if math.Abs(expected-actual) > delta { t.Errorf(...) }

## Error Assertions

	assert.Error(t, err)                        → if err == nil { t.Errorf(...) }
	assert.NoError(t, err)                      → if err != nil { t.Errorf(...) }
	assert.ErrorIs(t, err, target)              → if !errors.Is(err, target) { t.Errorf(...) }
	assert.ErrorAs(t, err, &target)             → if !errors.As(err, &target) { t.Errorf(...) }

## Require Package

	require.Equal(t, expected, actual)          → if got != want { t.Fatalf(...) }
	require.NoError(t, err)                     → if err != nil { t.Fatalf(...) }
	require.True(t, condition)                  → if !condition { t.Fatalf(...) }

All require assertions use t.Fatalf instead of t.Errorf to maintain the
fail-fast behavior.

## Suite Testing

	type MySuite struct {
		suite.Suite
	}                                           → type MySuite struct{}

	func TestMySuite(t *testing.T) {
		suite.Run(t, new(MySuite))
	}                                           → Individual Test* functions

	func (s *MySuite) TestSomething() {
		s.Equal(expected, actual)
	}                                           → func TestSomething(t *testing.T) { ... }

## Mock Package

The tool identifies mock usage but does not automatically convert it, instead
adding TODO comments suggesting alternative approaches:

	mock.Mock                                   → // TODO: Consider interface-based test doubles
	AssertExpectations                          → // TODO: Implement expectation verification

# Import Management

The tool automatically:
  - Removes unused testify imports
  - Adds required stdlib imports (testing, errors, strings, etc.)
  - Adds cmp imports when needed for complex comparisons
  - Preserves existing imports and their organization

# Limitations

The following patterns require manual intervention:

  - Custom testify matchers
  - Complex mock setups
  - Suite-level setup/teardown (converted to test-level)
  - Assertions in helper functions (may need *testing.T parameter)
  - Parallel test synchronization in suites

# Output

By default, the tool modifies files in place. Use -dry-run to preview changes
or -diff to see detailed diffs. The tool preserves file formatting using gofmt.

# Exit Codes

	0: Success, all files processed
	1: Command line argument error  
	2: File access or parsing error
	3: Transformation error
	4: File write error

# Philosophy

This tool embraces the Go philosophy of preferring standard library solutions.
While testify provides convenient assertion methods, the standard library's
explicit conditionals are:
  - Clearer about what's being tested
  - Easier to debug when tests fail
  - Free from external dependencies
  - More idiomatic Go code

The addition of the cmp package (likely to be promoted to stdlib) provides
powerful comparison capabilities for complex structures while maintaining
the simplicity of the standard testing approach.

Remember: de minimis non curat lex - the law doesn't concern itself with
trifles, and neither should your tests concern themselves with unnecessary
dependencies.
*/
package main