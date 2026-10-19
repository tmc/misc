package traceworkload

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestBlockOptions(t *testing.T) {
	moon := `{"session_id":"s","input_length":8,"output_length":2,"hash_ids":[1,2]}`
	for _, tt := range []struct {
		name string
		read func(io.Reader) (SessionTrace, error)
		want int
	}{
		{"moon default", ReadMooncake, 512},
		{"moon explicit4", func(r io.Reader) (SessionTrace, error) { return ReadMooncakeWithOptions(r, BlockOptions{BlockSize: 4}) }, 4},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.read(strings.NewReader(moon))
			if err != nil {
				t.Fatal(err)
			}
			s := got.Sessions[0]
			if s.BlockSize != tt.want || s.HashScope != "file" || !reflect.DeepEqual(s.Turns[0].HashIDs, []string{"1", "2"}) {
				t.Fatalf("declaration lost: %+v", s)
			}
		})
	}
	trace := `{"session_id":"s","round_index":0,"input_tokens_total":8,"prefix_tokens":0,"newly_append_tokens":8,"timing_events":[{"event_type":"user_message","timestamp":"2026-10-18T00:00:00Z"}]}`
	got, err := ReadTraceLab(strings.NewReader(trace))
	if err != nil || got.Sessions[0].BlockSize != 64 {
		t.Fatalf("default: %v", err)
	}
	for _, size := range []int{1, 4, 1000000} {
		got, err := ReadTraceLabWithOptions(strings.NewReader(trace), BlockOptions{BlockSize: size})
		if err != nil {
			t.Fatal(err)
		}
		if got.Sessions[0].BlockSize != size || len(got.Sessions[0].Turns[0].HashIDs) != 8/size {
			t.Fatal("virtual chain ignored block size")
		}
	}
	for _, size := range []int{0, -1, 1000001} {
		for _, read := range []func(io.Reader, BlockOptions) (SessionTrace, error){ReadMooncakeWithOptions, ReadTraceLabWithOptions} {
			if got, err := read(strings.NewReader("invalid JSON must not be consumed"), BlockOptions{BlockSize: size}); err == nil || !strings.Contains(err.Error(), "block size") || len(got.Sessions) != 0 {
				t.Fatalf("invalid size %d admitted: %v", size, err)
			}
		}
	}
	large := strings.ReplaceAll(trace, `"input_tokens_total":8`, `"input_tokens_total":1000000`)
	large = strings.ReplaceAll(large, `"newly_append_tokens":8`, `"newly_append_tokens":1000000`)
	if _, err := ReadTraceLabWithOptions(strings.NewReader(large), BlockOptions{BlockSize: 1}); err == nil || !strings.Contains(err.Error(), "aggregate virtual block budget") {
		t.Fatalf("small block bypassed preflight: %v", err)
	}
}

func TestTraceLabBlockFourOracle(t *testing.T) {
	f, err := os.Open("testdata/tracelab.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got, err := ReadTraceLabWithOptions(f, BlockOptions{BlockSize: 4})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("testdata/tracelab-block4-oracle.json")
	if err != nil {
		t.Fatal(err)
	}
	var want []struct {
		ID        string
		BlockSize int `json:"block_size"`
		Requests  []struct {
			HashIDs []int `json:"hash_ids"`
		}
	}
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	if len(got.Sessions) != len(want) {
		t.Fatal("group population")
	}
	for i, s := range got.Sessions {
		if s.ID != want[i].ID || s.BlockSize != want[i].BlockSize || len(s.Turns) != len(want[i].Requests) {
			t.Fatal("group declaration")
		}
		for j, turn := range s.Turns {
			ids := []string{}
			for _, n := range want[i].Requests[j].HashIDs {
				ids = append(ids, fmt.Sprint(n))
			}
			if !reflect.DeepEqual(turn.HashIDs, ids) {
				t.Fatalf("chain %d/%d: %v / %v", i, j, turn.HashIDs, ids)
			}
		}
	}
}

func ExampleReadMooncakeWithOptions() {
	trace, err := ReadMooncakeWithOptions(strings.NewReader(`{"input_length":8,"hash_ids":[1,2]}`), BlockOptions{BlockSize: 4})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(trace.Sessions[0].BlockSize, trace.Sessions[0].HashScope)
	// Output: 4 file
}
func ExampleReadTraceLabWithOptions() {
	input := `{"session_id":"one","round_index":0,"input_tokens_total":8,"prefix_tokens":0,"newly_append_tokens":8,"timing_events":[{"event_type":"user_message","timestamp":"2026-10-18T00:00:00Z"}]}`
	trace, err := ReadTraceLabWithOptions(strings.NewReader(input), BlockOptions{BlockSize: 4})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(trace.Sessions[0].Turns[0].HashIDs)
	// Output: [1 2]
}
