package serving_test

import (
	"context"
	"fmt"
	"github.com/tmc/misc/xk6-llm-perf/serving"
)

func ExampleRun() {
	receipt, err := serving.Run(context.Background(), serving.LifecycleConfig{})
	fmt.Println(receipt.Complete, err != nil)
	// Output: false true
}
