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

func TestVendorOracle(t *testing.T) {
	data, err := os.ReadFile("testdata/vendor-oracle.json")
	if err != nil {
		t.Fatal(err)
	}
	var oracle struct {
		Mooncake []struct {
			ID, Mode string
			Turns    []struct {
				Timestamp, Delay *float64
				Input, Output    *int
				Hashes           []string
			}
		}
		Bailian []struct {
			ID, Mode string
			Turns    []struct {
				ID            string
				Timestamp     *float64
				Input, Output *int
				Hashes        []string
			}
		}
		Tracelab []struct {
			ID        string
			BlockSize int `json:"block_size"`
			Requests  []struct {
				T          float64
				In, Out    int
				HashIDs    []int `json:"hash_ids"`
				Stop       string
				InputTypes []string `json:"input_types"`
			}
		}
	}
	// Separate timing fields avoid a shared JSON tag on grouped declarations.
	if err := json.Unmarshal(data, &oracle); err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		read func(io.Reader) (SessionTrace, error)
	}{{"mooncake", ReadMooncake}, {"bailian", ReadBailian}, {"tracelab", ReadTraceLab}} {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.Open("testdata/" + tt.name + ".jsonl")
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			got, err := tt.read(f)
			if err != nil {
				t.Fatal(err)
			}
			expected := len(oracle.Mooncake)
			if tt.name == "bailian" {
				expected = len(oracle.Bailian)
			}
			if tt.name == "tracelab" {
				expected = len(oracle.Tracelab)
			}
			if len(got.Sessions) != expected {
				t.Fatal("session population changed")
			}
			if len(got.SourceHash) != 64 {
				t.Fatal("missing source identity")
			}
			for i, s := range got.Sessions {
				switch tt.name {
				case "mooncake":
					want := oracle.Mooncake[i]
					if s.ID != want.ID || len(s.Turns) != len(want.Turns) {
						t.Fatal("session grouping")
					}
					mode := "live"
					if want.Mode == "message_array_with_responses" {
						mode = "recorded"
					}
					if s.ContextMode != mode {
						t.Fatalf("context: %s / %s", s.ContextMode, want.Mode)
					}
					for j, v := range s.Turns {
						w := want.Turns[j]
						if !reflect.DeepEqual(v.Timestamp, w.Timestamp) || !reflect.DeepEqual(v.Delay, w.Delay) || !reflect.DeepEqual(v.InputTokens, w.Input) || !reflect.DeepEqual(v.OutputTokens, w.Output) || !reflect.DeepEqual(v.HashIDs, w.Hashes) {
							t.Fatalf("turn %d: %+v / %+v", j, v, w)
						}
					}
				case "bailian":
					want := oracle.Bailian[i]
					if s.ID != want.ID || s.BlockSize != 16 || s.ContextMode != "recorded" || len(s.Turns) != len(want.Turns) {
						t.Fatal("session grouping")
					}
					for j, v := range s.Turns {
						w := want.Turns[j]
						if v.ID != w.ID || !reflect.DeepEqual(v.Timestamp, w.Timestamp) || !reflect.DeepEqual(v.InputTokens, w.Input) || !reflect.DeepEqual(v.OutputTokens, w.Output) || !reflect.DeepEqual(v.HashIDs, w.Hashes) {
							t.Fatalf("turn %d: %+v / %+v", j, v, w)
						}
					}
				case "tracelab":
					want := oracle.Tracelab[i]
					if s.ID != want.ID || s.BlockSize != want.BlockSize || len(s.Turns) != len(want.Requests) {
						t.Fatal("session grouping")
					}
					var refs []struct {
						Requests []struct {
							APITime   *float64 `json:"api_time"`
							ThinkTime *float64 `json:"think_time"`
						}
					}
					if err := json.Unmarshal(raw["tracelab"], &refs); err != nil {
						t.Fatal(err)
					}
					for j, v := range s.Turns {
						w := want.Requests[j]
						hashes := []string{}
						for _, h := range w.HashIDs {
							hashes = append(hashes, fmt.Sprint(h))
						}
						if *v.Timestamp != w.T*1000 || *v.InputTarget != w.In || *v.OutputBudget != w.Out || !reflect.DeepEqual(v.HashIDs, hashes) || v.Stop != w.Stop || v.InputType != w.InputTypes[0] {
							t.Fatalf("turn %d: %+v / %+v", j, v, w)
						}
						for _, pair := range [][2]*float64{{v.APITime, refs[i].Requests[j].APITime}, {v.ThinkTime, refs[i].Requests[j].ThinkTime}} {
							if pair[0] == nil || pair[1] == nil {
								if pair[0] != nil || pair[1] != nil {
									t.Fatal("missing proxy")
								}
							} else if *pair[0] != *pair[1]*1000 {
								t.Fatalf("timing proxy: %v / %v", *pair[0], *pair[1])
							}
						}
					}
				}
			}
			var again SessionTrace
			b, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(b, &again); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, again) {
				t.Fatal("normalized artifact round trip")
			}
		})
	}
}

func TestVendorRefusals(t *testing.T) {
	tests := []struct {
		name, input string
		read        func(io.Reader) (SessionTrace, error)
	}{
		{"missing mode", `{}`, ReadMooncake}, {"mixed mode", `{"input_length":2,"text_input":"x"}`, ReadMooncake}, {"bad live", `{"input_length":2,"assistant_responses":"live"}`, ReadMooncake}, {"empty messages", `{"messages":[]}`, ReadMooncake}, {"empty payload", `{"payload":{}}`, ReadMooncake},
		{"mixed session", `{"session_id":"x","messages":[{"role":"user"}],"assistant_responses":"live"}` + "\n" + `{"session_id":"x","messages":[{"role":"user"}]}`, ReadMooncake},
		{"null", `{"input_length":null}`, ReadMooncake}, {"duplicate", `{"input_length":1,"input_length":2}`, ReadMooncake}, {"surrogate", `{"text_input":"\ud800"}`, ReadMooncake}, {"negative", `{"input_length":-1}`, ReadMooncake}, {"fraction", `{"input_length":1.5}`, ReadMooncake}, {"bad hash", `{"input_length":2,"hash_ids":[1.5]}`, ReadMooncake}, {"trailing", `{"input_length":2} {}`, ReadMooncake}, {"time unit cap", `{"input_length":2,"timestamp":86400001}`, ReadMooncake},
		{"missing bailian", `{}`, ReadBailian}, {"bad chat", `{"chat_id":"bad","timestamp":1,"input_length":1,"output_length":1}`, ReadBailian},
		{"dangling", `{"chat_id":1,"parent_chat_id":2,"timestamp":0,"input_length":1,"output_length":1}`, ReadBailian},
		{"cycle", `{"chat_id":1,"parent_chat_id":1,"timestamp":0,"input_length":1,"output_length":1}`, ReadBailian},
		{"duplicate chat", strings.Repeat(`{"chat_id":1,"timestamp":0,"input_length":1,"output_length":1}`+"\n", 2), ReadBailian},
		{"bailian day", `{"chat_id":1,"timestamp":86401,"input_length":1,"output_length":1}`, ReadBailian},
		{"undated", `{"session_id":"s","round_index":0,"input_tokens_total":1,"prefix_tokens":0,"newly_append_tokens":1,"timing_events":[]}`, ReadTraceLab},
		{"prefix mismatch", `{"session_id":"s","round_index":0,"input_tokens_total":1,"prefix_tokens":2,"newly_append_tokens":0,"timing_events":[{"event_type":"text","timestamp":"2026-10-18T00:00:00Z"}]}`, ReadTraceLab},
		{"subagent", `{"session_id":"s","round_index":0,"input_tokens_total":1,"prefix_tokens":0,"newly_append_tokens":1,"tools":[{"tool_name":"Agent"}],"timing_events":[{"event_type":"text","timestamp":"2026-10-18T00:00:00Z"}]}`, ReadTraceLab},
		{"size", strings.Repeat(" ", maxBytes+1), ReadMooncake},
		{"session turn cap", strings.Repeat(`{"session_id":"x","input_length":1}`+"\n", 1001), ReadMooncake},
		{"depth", `{"input_length":1,"ancillary":` + strings.Repeat("[", 65) + `0` + strings.Repeat("]", 65) + `}`, ReadMooncake},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := tt.read(strings.NewReader(tt.input)); err == nil || len(got.Sessions) != 0 {
				t.Fatalf("invalid input admitted: %v", err)
			}
		})
	}
}

func ExampleReadMooncake() {
	trace, err := ReadMooncake(strings.NewReader(`{"session_id":"one","text_input":"hello","output_length":8}`))
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(trace.Sessions[0].ID, *trace.Sessions[0].Turns[0].TextInput)
	// Output: one hello
}
func ExampleReadBailian() {
	trace, err := ReadBailian(strings.NewReader(`{"chat_id":3,"timestamp":1.25,"input_length":16,"output_length":4,"hash_ids":[7]}`))
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(trace.Sessions[0].ID, *trace.Sessions[0].Turns[0].Timestamp)
	// Output: 3 1250
}
func ExampleReadTraceLab() {
	trace, err := ReadTraceLab(strings.NewReader(`{"session_id":"one","round_index":0,"input_tokens_total":64,"prefix_tokens":0,"newly_append_tokens":64,"output_tokens":2,"timing_events":[{"event_type":"user_message","timestamp":"2026-10-18T00:00:00Z"}]}`))
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(trace.Sessions[0].Turns[0].HashIDs)
	// Output: [1]
}

func TestAggregateBudgets(t *testing.T) {
	row := `{"session_id":"x","round_index":0,"input_tokens_total":1000000,"prefix_tokens":0,"newly_append_tokens":1000000,"output_tokens":1,"timing_events":[{"event_type":"user_message","timestamp":"2026-10-18T00:00:00Z"}]}` + "\n"
	if got, err := ReadTraceLab(strings.NewReader(strings.Repeat(row, 17))); err == nil || !strings.Contains(err.Error(), "aggregate virtual block budget") || len(got.Sessions) != 0 {
		t.Fatalf("expansion admitted: %v", err)
	}
	hashes := `{"input_length":1,"hash_ids":[` + strings.Repeat("1,", maxCacheBlocks) + `1]}`
	if _, err := ReadMooncake(strings.NewReader(hashes)); err == nil || !strings.Contains(err.Error(), "aggregate cache block budget") {
		t.Fatalf("hash allocation admitted: %v", err)
	}
	if _, err := ReadMooncake(strings.NewReader(strings.Repeat(`{"input_length":1}`+"\n", 20000))); err == nil || !strings.Contains(err.Error(), "aggregate output budget") {
		t.Fatalf("output amplification admitted: %v", err)
	}
	small := strings.ReplaceAll(row, "1000000", "128")
	got, err := ReadTraceLab(strings.NewReader(strings.Repeat(small, 2)))
	if err != nil || len(got.Sessions) != 1 || len(got.Sessions[0].Turns[1].HashIDs) != 2 {
		t.Fatalf("valid small trace: %v", err)
	}
}
