package main

import (
	"encoding/json"
	"fmt"
	"time"

	. "github.com/tmc/misc/rrweb"
)

// Sample document creation
func main() {
	// Initialize the base timestamp
	baseTime := time.Now().UnixMilli() // Convert to int64

	// Create a full snapshot event
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
									Type: Element,
									TagName: "pre",
									Attributes: map[string]string{},
									ChildNodes: []SerializedNodeWithId{
										{
											SerializedNode: SerializedNode{
												Type:        Text,
												TextContent: "shell> ",
											},
											ID: 3,
										},
									},
								},
								ID: 2,
							},
						},
					},
					ID: 1,
				},
				InitialOffset: InitialOffset{
					Top:  0,
					Left: 0,
				},
			},
		},
		Timestamp: baseTime,
	}

	// Create a viewport resize event
	viewportResizeEvent := EventWithTime{
		EventWithoutTime: EventWithoutTime{
			Type: IncrementalSnapshot,
			Data: ViewportResizeData{
				Source: ViewportResize,
				Width:  1024,
				Height: 768,
			},
		},
		Timestamp: baseTime + 2000, // Add 2 seconds in milliseconds
	}

	// Simulate shell session events
	shellCommands := []string{
		"ls\n",
		"file1.txt\nfile2.txt\nfile3.txt\n",
	}

	var shellEvents []EventWithTime
	currentTextContent := "shell> "
	for i, command := range shellCommands {
		for j, char := range command {
			currentTextContent += string(char)
			event := EventWithTime{
				EventWithoutTime: EventWithoutTime{
					Type: IncrementalSnapshot,
					Data: TextMutationData{
						Source: Input,
						ID:     3,
						Text:   currentTextContent,
					},
				},
				Timestamp: baseTime + int64((3+i*len(command)+j)*500), // Increment time with slight delay
			}
			shellEvents = append(shellEvents, event)
		}
	}

	// Combine events into a recording
	recording := []EventWithTime{
		fullSnapshotEvent,
		viewportResizeEvent,
	}
	recording = append(recording, shellEvents...)

	// Convert recording to JSON for output
	recordingJSON, err := json.MarshalIndent(recording, "", "  ")
	if err != nil {
		fmt.Println("Error marshalling recording:", err)
		return
	}

	// Print the recording
	fmt.Println(string(recordingJSON))
}
