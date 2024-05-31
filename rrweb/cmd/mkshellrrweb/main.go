package main

import (
	"encoding/json"
	"fmt"
	"time"

	. "github.com/tmc/misc/rrweb"
)

func main() {
	// Initialize base timestamp
	baseTime := time.Now().UnixNano() / int64(time.Millisecond)

	// Meta event
	metaEvent := EventWithTime{
		EventWithoutTime: EventWithoutTime{
			Type: Meta,
			Data: MetaEventData{
				Href:   "file:///Users/tmc/go/src/github.com/tmc/misc/rrweb/testdata/gentestdata1.html",
				Width:  1424,
				Height: 1236,
			},
		},
		Timestamp: baseTime,
	}

	// Full snapshot event
	fullSnapshotEvent := EventWithTime{
		EventWithoutTime: EventWithoutTime{
			Type: FullSnapshot,
			Data: FullSnapshotEventData{
				Node: SerializedNodeWithId{
					SerializedNode: SerializedNode{
						Type: Document,
						ChildNodes: []SerializedNodeWithId{
							{
								SerializedNode: SerializedNode{
									Type:      DocumentType,
									Name:      "html",
									PublicId:  "",
									SystemId:  "",
								},
								ID: 2,
							},
							{
								SerializedNode: SerializedNode{
									Type:      Element,
									TagName:   "html",
									Attributes: map[string]string{"lang": "en"},
									ChildNodes: []SerializedNodeWithId{
										{
											SerializedNode: SerializedNode{
												Type:      Element,
												TagName:   "head",
												ChildNodes: []SerializedNodeWithId{
													{
														SerializedNode: SerializedNode{
															Type: Text,
															TextContent: "\n    ",
														},
														ID: 5,
													},
													{
														SerializedNode: SerializedNode{
															Type: Element,
															TagName: "meta",
															Attributes: map[string]string{"charset": "UTF-8"},
														},
														ID: 6,
													},
													{
														SerializedNode: SerializedNode{
															Type: Text,
															TextContent: "\n    ",
														},
														ID: 7,
													},
													{
														SerializedNode: SerializedNode{
															Type: Element,
															TagName: "meta",
															Attributes: map[string]string{
																"name": "viewport",
																"content": "width=device-width, initial-scale=1.0",
															},
														},
														ID: 8,
													},
													{
														SerializedNode: SerializedNode{
															Type: Text,
															TextContent: "\n    ",
														},
														ID: 9,
													},
													{
														SerializedNode: SerializedNode{
															Type: Element,
															TagName: "title",
															ChildNodes: []SerializedNodeWithId{
																{
																	SerializedNode: SerializedNode{
																		Type: Text,
																		TextContent: "Simulated Terminal with Automatic Recording",
																	},
																	ID: 11,
																},
															},
														},
														ID: 10,
													},
													{
														SerializedNode: SerializedNode{
															Type: Text,
															TextContent: "\n    ",
														},
														ID: 12,
													},
													{
														SerializedNode: SerializedNode{
															Type: Element,
															TagName: "script",
															Attributes: map[string]string{
																"src": "https://cdn.jsdelivr.net/npm/rrweb@latest/dist/record/rrweb-record.min.js",
															},
														},
														ID: 13,
													},
													{
														SerializedNode: SerializedNode{
															Type: Text,
															TextContent: "\n    ",
														},
														ID: 14,
													},
													{
														SerializedNode: SerializedNode{
															Type: Element,
															TagName: "style",
															ChildNodes: []SerializedNodeWithId{
																{
																	SerializedNode: SerializedNode{
																		Type: Text,
																		TextContent: "body { font-family: \"Courier New\", Courier, monospace; background-color: black; color: green; margin: 0px; display: flex; flex-direction: column; align-items: center; justify-content: center; height: 100vh; } #terminal-container { width: 80%; height: 60%; background-color: black; border: 1px solid green; padding: 10px; box-sizing: border-box; overflow-y: auto; } #terminal { white-space: pre-wrap; }",
																	},
																	ID: 16,
																},
															},
														},
														ID: 15,
													},
													{
														SerializedNode: SerializedNode{
															Type: Text,
															TextContent: "\n",
														},
														ID: 17,
													},
												},
											},
											ID: 4,
										},
										{
											SerializedNode: SerializedNode{
												Type: Text,
												TextContent: "\n",
											},
											ID: 18,
										},
										{
											SerializedNode: SerializedNode{
												Type: Element,
												TagName: "body",
												ChildNodes: []SerializedNodeWithId{
													{
														SerializedNode: SerializedNode{
															Type: Text,
															TextContent: "\n    ",
														},
														ID: 20,
													},
													{
														SerializedNode: SerializedNode{
															Type: Element,
															TagName: "div",
															Attributes: map[string]string{
																"id": "terminal-container",
															},
															ChildNodes: []SerializedNodeWithId{
																{
																	SerializedNode: SerializedNode{
																		Type: Text,
																		TextContent: "\n        ",
																	},
																	ID: 22,
																},
																{
																	SerializedNode: SerializedNode{
																		Type: Element,
																		TagName: "div",
																		Attributes: map[string]string{
																			"id": "terminal",
																		},
																	},
																	ID: 23,
																},
																{
																	SerializedNode: SerializedNode{
																		Type: Text,
																		TextContent: "\n    ",
																	},
																	ID: 24,
																},
															},
														},
														ID: 21,
													},
													{
														SerializedNode: SerializedNode{
															Type: Text,
															TextContent: "\n\n    ",
														},
														ID: 25,
													},
													{
														SerializedNode: SerializedNode{
															Type: Element,
															TagName: "script",
															ChildNodes: []SerializedNodeWithId{
																{
																	SerializedNode: SerializedNode{
																		Type: Text,
																		TextContent: "SCRIPT_PLACEHOLDER",
																	},
																	ID: 27,
																},
															},
														},
														ID: 26,
													},
													{
														SerializedNode: SerializedNode{
															Type: Text,
															TextContent: "\n\n\n",
														},
														ID: 28,
													},
												},
											},
											ID: 19,
										},
									},
								},
								ID: 3,
							},
						},
					},
				},
				InitialOffset: Offset{
					Top:  0,
					Left: 0,
				},
			},
		},
		Timestamp: baseTime + 2000,
	}

	// Simulate terminal commands with incremental updates
	textEvents := []EventWithTime{
		{
			EventWithoutTime: EventWithoutTime{
				Type: IncrementalSnapshot,
				Data: TextMutation{
					Source: 5,
					Texts: []TextMutationData{
						{ID: 23, Text: "shell\u003e l"},
					},
				},
			},
			Timestamp: baseTime + 4000,
		},
		{
			EventWithoutTime: EventWithoutTime{
				Type: IncrementalSnapshot,
				Data: TextMutation{
					Source: 5,
					Texts: []TextMutationData{
						{ID: 23, Text: "shell\u003e ls"},
					},
				},
			},
			Timestamp: baseTime + 6000,
		},
		{
			EventWithoutTime: EventWithoutTime{
				Type: IncrementalSnapshot,
				Data: TextMutation{
					Source: 5,
					Texts: []TextMutationData{
						{ID: 23, Text: "shell\u003e ls\n"},
					},
				},
			},
			Timestamp: baseTime + 8000,
		},
		{
			EventWithoutTime: EventWithoutTime{
				Type: IncrementalSnapshot,
				Data: TextMutation{
					Source: 5,
					Texts: []TextMutationData{
						{ID: 23, Text: "shell\u003e ls\nfile1"},
					},
				},
			},
			Timestamp: baseTime + 10000,
		},
		{
			EventWithoutTime: EventWithoutTime{
				Type: IncrementalSnapshot,
				Data: TextMutation{
					Source: 5,
					Texts: []TextMutationData{
						{ID: 23, Text: "shell\u003e ls\nfile1.txt"},
					},
				},
			},
			Timestamp: baseTime + 12000,
		},
		{
			EventWithoutTime: EventWithoutTime{
				Type: IncrementalSnapshot,
				Data: TextMutation{
					Source: 5,
					Texts: []TextMutationData{
						{ID: 23, Text: "shell\u003e ls\nfile1.txt\n"},
					},
				},
			},
			Timestamp: baseTime + 14000,
		},
		{
			EventWithoutTime: EventWithoutTime{
				Type: IncrementalSnapshot,
				Data: TextMutation{
					Source: 5,
					Texts: []TextMutationData{
						{ID: 23, Text: "shell\u003e ls\nfile1.txt\nfile2"},
					},
				},
			},
			Timestamp: baseTime + 16000,
		},
		{
			EventWithoutTime: EventWithoutTime{
				Type: IncrementalSnapshot,
				Data: TextMutation{
					Source: 5,
					Texts: []TextMutationData{
						{ID: 23, Text: "shell\u003e ls\nfile1.txt\nfile2.txt"},
					},
				},
			},
			Timestamp: baseTime + 18000,
		},
		{
			EventWithoutTime: EventWithoutTime{
				Type: IncrementalSnapshot,
				Data: TextMutation{
					Source: 5,
					Texts: []TextMutationData{
						{ID: 23, Text: "shell\u003e ls\nfile1.txt\nfile2.txt\n"},
					},
				},
			},
			Timestamp: baseTime + 20000,
		},
		{
			EventWithoutTime: EventWithoutTime{
				Type: IncrementalSnapshot,
				Data: TextMutation{
					Source: 5,
					Texts: []TextMutationData{
						{ID: 23, Text: "shell\u003e ls\nfile1.txt\nfile2.txt\nfile3"},
					},
					},
				},
			Timestamp: baseTime + 22000,
		},
		{
			EventWithoutTime: EventWithoutTime{
				Type: IncrementalSnapshot,
				Data: TextMutation{
					Source: 5,
					Texts: []TextMutationData{
						{ID: 23, Text: "shell\u003e ls\nfile1.txt\nfile2.txt\nfile3.txt"},
					},
				},
			},
			Timestamp: baseTime + 24000,
		},
		{
			EventWithoutTime: EventWithoutTime{
				Type: IncrementalSnapshot,
				Data: TextMutation{
					Source: 5,
					Texts: []TextMutationData{
						{ID: 23, Text: "shell\u003e ls\nfile1.txt\nfile2.txt\nfile3.txt\n"},
					},
				},
			},
			Timestamp: baseTime + 26000,
		},
		{
			EventWithoutTime: EventWithoutTime{
				Type: IncrementalSnapshot,
				Data: TextMutation{
					Source: 5,
					Texts: []TextMutationData{
						{ID: 23, Text: "shell\u003e echo 'Hello World!'"},
					},
				},
			},
			Timestamp: baseTime + 28000,
		},
		{
			EventWithoutTime: EventWithoutTime{
				Type: IncrementalSnapshot,
				Data: TextMutation{
					Source: 5,
					Texts: []TextMutationData{
						{ID: 23, Text: "shell\u003e echo 'Hello World!'\n"},
					},
				},
			},
			Timestamp: baseTime + 30000,
		},
		{
			EventWithoutTime: EventWithoutTime{
				Type: IncrementalSnapshot,
				Data: TextMutation{
					Source: 5,
					Texts: []TextMutationData{
						{ID: 23, Text: "shell\u003e cat file1.txt"},
					},
				},
			},
			Timestamp: baseTime + 32000,
		},
		{
			EventWithoutTime: EventWithoutTime{
				Type: IncrementalSnapshot,
				Data: TextMutation{
					Source: 5,
					Texts: []TextMutationData{
						{ID: 23, Text: "shell\u003e cat file1.txt\n"},
					},
				},
			},
			Timestamp: baseTime + 34000,
		},
	}

	// Combine all events
	events := append([]EventWithTime{metaEvent, fullSnapshotEvent}, textEvents...)

	// Encode to JSON
	output, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		fmt.Println("Error marshalling events:", err)
		return
	}

	fmt.Println(string(output))
}
