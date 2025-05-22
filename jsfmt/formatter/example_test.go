package formatter_test

import (
	"fmt"
	"os"

	"github.com/tmc/misc/jsfmt/formatter"
)

func ExampleFormat() {
	// Input JavaScript code with inconsistent formatting
	input := []byte(`function example(){
if(condition){
doSomething();}else{
doSomethingElse();
}
const x=1+2;
}`)

	// Format the code
	formatted, err := formatter.Format("example.js", input, 2)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	fmt.Println(string(formatted))
	// Output:
	// function example() {
	//   if (condition) {
	//     doSomething();
	//   } else {
	//     doSomethingElse();
	//   }
	//   const x = 1 + 2;
	// }
}