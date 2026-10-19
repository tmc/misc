package traceworkload_test

import (
	"fmt"
	"github.com/tmc/misc/xk6-llm-perf/traceworkload"
	"strings"
)

func ExampleReadBurstGPT() {
	trace, err := traceworkload.ReadBurstGPT(strings.NewReader("Timestamp,Request tokens,Response tokens\n1.25,11,4\n"))
	if err != nil {
		fmt.Println(err)
		return
	}
	row := trace.Rows[0]
	fmt.Println(row.Offset, row.InputTokens, row.OutputBudget, row.SourceRow)
	fmt.Println(trace.TimestampUnit, trace.OffsetUnit, len(trace.SourceHash))
	// Output:
	// 1250 11 4 1
	// seconds milliseconds 64
}
