package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type RRWebEvent struct {
	Type      int         `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp int64       `json:"timestamp"`
}

type ViewportResizeData struct {
	Href   string `json:"href"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type SerializedNode struct {
	Type       int               `json:"type"`
	Name       string            `json:"name,omitempty"`
	PublicID   string            `json:"publicId,omitempty"`
	SystemID   string            `json:"systemId,omitempty"`
	TagName    string            `json:"tagName,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
	ChildNodes []SerializedNode  `json:"childNodes,omitempty"`
	ID         int               `json:"id"`
	TextContent string           `json:"textContent,omitempty"`
	IsStyle     bool             `json:"isStyle,omitempty"`
}

type FullSnapshotData struct {
	Node SerializedNode `json:"node"`
	InitialOffset struct {
		Top  int `json:"top"`
		Left int `json:"left"`
	} `json:"initialOffset"`
}

type MutationData struct {
	Source    int         `json:"source"`
	Texts     []TextMutation `json:"texts,omitempty"`
}

type TextMutation struct {
	ID int    `json:"id"`
	Text string `json:"text"`
}

func main() {
	timestamp := time.Now().UnixNano() / int64(time.Millisecond)
	events := []RRWebEvent{
		{
			Type: 4,
			Data: ViewportResizeData{
				Href:   "file:///Users/tmc/go/src/github.com/tmc/misc/rrweb/testdata/gentestdata1.html",
				Width:  1424,
				Height: 1236,
			},
			Timestamp: timestamp,
		},
		{
			Type: 2,
			Data: FullSnapshotData{
				Node: SerializedNode{
					Type: 0,
					ChildNodes: []SerializedNode{
						{
							Type: 1,
							Name: "html",
							PublicID: "",
							SystemID: "",
							ID: 2,
						},
						{
							Type: 2,
							TagName: "html",
							Attributes: map[string]string{
								"lang": "en",
							},
							ChildNodes: []SerializedNode{
								{
									Type: 2,
									TagName: "head",
									ChildNodes: []SerializedNode{
										{
											Type: 3,
											TextContent: "\n    ",
											ID: 5,
										},
										{
											Type: 2,
											TagName: "meta",
											Attributes: map[string]string{
												"charset": "UTF-8",
											},
											ID: 6,
										},
										{
											Type: 3,
											TextContent: "\n    ",
											ID: 7,
										},
										{
											Type: 2,
											TagName: "meta",
											Attributes: map[string]string{
												"name": "viewport",
												"content": "width=device-width, initial-scale=1.0",
											},
											ID: 8,
										},
										{
											Type: 3,
											TextContent: "\n    ",
											ID: 9,
										},
										{
											Type: 2,
											TagName: "title",
											ChildNodes: []SerializedNode{
												{
													Type: 3,
													TextContent: "Simulated Terminal with Automatic Recording",
													ID: 11,
												},
											},
											ID: 10,
										},
										{
											Type: 3,
											TextContent: "\n    ",
											ID: 12,
										},
										{
											Type: 2,
											TagName: "script",
											Attributes: map[string]string{
												"src": "https://cdn.jsdelivr.net/npm/rrweb@latest/dist/record/rrweb-record.min.js",
											},
											ID: 13,
										},
										{
											Type: 3,
											TextContent: "\n    ",
											ID: 14,
										},
										{
											Type: 2,
											TagName: "style",
											ChildNodes: []SerializedNode{
												{
													Type: 3,
													TextContent: `body { font-family: "Courier New", Courier, monospace; background-color: black; color: green; margin: 0px; display: flex; flex-direction: column; align-items: center; justify-content: center; height: 100vh; } #terminal-container { width: 80%; height: 60%; background-color: black; border: 1px solid green; padding: 10px; box-sizing: border-box; overflow-y: auto; } #terminal { white-space: pre-wrap; }`,
													IsStyle: true,
													ID: 16,
												},
											},
											ID: 15,
										},
										{
											Type: 3,
											TextContent: "\n",
											ID: 17,
										},
									},
									ID: 4,
								},
								{
									Type: 3,
									TextContent: "\n",
									ID: 18,
								},
								{
									Type: 2,
									TagName: "body",
									ChildNodes: []SerializedNode{
										{
											Type: 3,
											TextContent: "\n    ",
											ID: 20,
										},
										{
											Type: 2,
											TagName: "div",
											Attributes: map[string]string{
												"id": "terminal-container",
											},
											ChildNodes: []SerializedNode{
												{
													Type: 3,
													TextContent: "\n        ",
													ID: 22,
												},
												{
													Type: 2,
													TagName: "div",
													Attributes: map[string]string{
														"id": "terminal",
													},
													ID: 23,
												},
												{
													Type: 3,
													TextContent: "\n    ",
													ID: 24,
												},
											},
											ID: 21,
										},
										{
											Type: 3,
											TextContent: "\n\n    ",
											ID: 25,
										},
										{
											Type: 2,
											TagName: "script",
											ChildNodes: []SerializedNode{
												{
													Type: 3,
													TextContent: "SCRIPT_PLACEHOLDER",
													ID: 27,
												},
											},
											ID: 26,
										},
										{
											Type: 3,
											TextContent: "\n\n\n",
											ID: 28,
										},
									},
									ID: 19,
								},
							},
							ID: 3,
						},
					},
					ID: 1,
				},
				InitialOffset: struct {
					Top  int `json:"top"`
					Left int `json:"left"`
				}{Top: 0, Left: 0},
			},
			Timestamp: timestamp + 2000,
		},
	}

	texts := []string{"l", "s", "\n", "file1", ".txt", "\n", "file2", ".txt", "\n", "file3", ".txt", "\n", "echo 'Hello World!'", "\n", "cat file1.txt", "\n"}

	for i, text := range texts {
		event := RRWebEvent{
			Type: 3,
			Data: MutationData{
				Source: 5,
				Texts: []TextMutation{
					{ID: 23, Text: "shell\u003e " + text},
				},
			},
			Timestamp: timestamp + int64(2000*(i+2)),
		}
		events = append(events, event)
	}

	output, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		fmt.Println("Error marshalling events:", err)
		return
	}

	err = os.WriteFile("recording.json", output, 0644)
	if err != nil {
		fmt.Println("Error writing to file:", err)
		return
	}

	fmt.Println("Recording JSON created successfully")
}
