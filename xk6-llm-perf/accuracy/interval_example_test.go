package accuracy_test

import (
	"fmt"
	"github.com/tmc/misc/xk6-llm-perf/accuracy"
)

func ExampleWilsonInterval() {
	r, err := accuracy.WilsonInterval(1, 4, .95)
	fmt.Printf("%d/%d: %.3f to %.3f (%v)\n", r.Correct, r.Total, r.Lower, r.Upper, err)
	// Output: 1/4: 0.046 to 0.699 (<nil>)
}
