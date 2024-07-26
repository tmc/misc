# show-least-covered

`show-least-covered` is a Go program that reads a Go test coverage profile, analyzes the source code of the current package, and prints out the function or method with the least coverage.

## Installation

1. Ensure you have Go installed on your system.
2. Clone this repository or download the source code.
3. Navigate to the project directory.
4. Run `go build` to compile the program.

## Usage

```
./show-least-covered [-cover coverage_profile_path]
```

- `-cover`: Path to the coverage profile (default: "coverage.out")

Example:

```
./show-least-covered -cover my_coverage.out
```

If no coverage profile is specified, the program will look for a file named "coverage.out" in the current directory.

## How it works

1. The program reads the specified coverage profile.
2. It analyzes the Go source files in the current package.
3. For each function and method, it calculates the coverage percentage.
4. Finally, it prints the name and coverage percentage of the function or method with the least coverage.

## License

This project is licensed under the MIT License. See the LICENSE file for details.

