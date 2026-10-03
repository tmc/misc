package echo_test

import (
	"fmt"
	"github.com/tmc/misc/wasmbind/examples/echo"
)

func Example() {
	fmt.Println(echo.Echo([]byte{1, 2, 3})) // Output: [1 2 3]
}
func ExampleEcho() {
	fmt.Println(echo.Echo([]byte{4, 5})) // Output: [4 5]
}
