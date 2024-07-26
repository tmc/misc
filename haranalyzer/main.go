package main

import (
	"context"
	_ "embed"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
)

//go:embed system-prompt.txt
var systemPrompt string

type HAREntry struct {
	StartedDateTime time.Time
	Method          string
	URL             string
	Status          int
	ResponseTime    float64
	ResponseSize    int64
	ContentType     string
}

type FilterFunc func(HAREntry) bool

type Config struct {
	InputFile    string
	OutputFormat string
	SortBy       string
	ChunkSize    int
	QueryString  string
	PerformAI    bool
	AnthropicKey string
	FilterFuncs  []FilterFunc
	CombineLogic string
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func run() error {
	var config Config

	rootCmd := &cobra.Command{
		Use:   "haranalyzer",
		Short: "Analyze HAR files with advanced filtering capabilities",
		RunE: func(cmd *cobra.Command, args []string) error {
			return analyzeHAR(config)
		},
	}

	rootCmd.Flags().StringVarP(&config.InputFile, "input", "i", "", "Input HAR file (required)")
	rootCmd.Flags().StringVarP(&config.OutputFormat, "output", "o", "text", "Output format (text, json, csv)")
	rootCmd.Flags().StringVarP(&config.SortBy, "sort", "s", "time", "Sort entries by (time, size, status, url)")
	rootCmd.Flags().IntVarP(&config.ChunkSize, "chunk", "c", 100, "Chunk size for analysis")
	rootCmd.Flags().StringVarP(&config.QueryString, "query", "q", "", "Query string for filtering")
	rootCmd.Flags().BoolVarP(&config.PerformAI, "ai", "a", false, "Perform AI analysis")
	rootCmd.Flags().StringVar(&config.AnthropicKey, "anthropic-key", "", "Anthropic API key")

	rootCmd.MarkFlagRequired("input")

	return rootCmd.Execute()
}

func analyzeHAR(config Config) error {
	entries, err := parseHARFile(config.InputFile)
	if err != nil {
		return fmt.Errorf("error parsing HAR file: %w", err)
	}

	if config.QueryString != "" {
		filterFuncs, combineLogic, err := parseQueryString(config.QueryString)
		if err != nil {
			return fmt.Errorf("error parsing query string: %w", err)
		}
		config.FilterFuncs = filterFuncs
		config.CombineLogic = combineLogic
	}

	filteredEntries := filterEntries(entries, config.FilterFuncs, config.CombineLogic)

	sortEntries(filteredEntries, config.SortBy)

	chunks := chunkEntries(filteredEntries, config.ChunkSize)

	for i, chunk := range chunks {
		summary := analyzeSummary(chunk)
		outputSummary(summary, i+1, config.OutputFormat)

		if config.PerformAI {
			aiAnalysis, err := performAIAnalysis(chunk, config.AnthropicKey)
			if err != nil {
				log.Printf("Error performing AI analysis: %v", err)
			} else {
				fmt.Printf("AI Analysis for Chunk %d:\n%s\n", i+1, aiAnalysis)
			}
		}
	}

	return nil
}

func parseHARFile(filename string) ([]HAREntry, error) {
	// Implement HAR file parsing logic here
	// This is a placeholder implementation
	return []HAREntry{}, nil
}

func parseQueryString(query string) ([]FilterFunc, string, error) {
	// Implement query string parsing logic here
	// This is a placeholder implementation
	return []FilterFunc{}, "AND", nil
}

func filterEntries(entries []HAREntry, filters []FilterFunc, logic string) []HAREntry {
	filtered := make([]HAREntry, 0)
	for _, entry := range entries {
		if applyFilters(entry, filters, logic) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func applyFilters(entry HAREntry, filters []FilterFunc, logic string) bool {
	if len(filters) == 0 {
		return true
	}

	results := make([]bool, len(filters))
	for i, filter := range filters {
		results[i] = filter(entry)
	}

	if logic == "AND" {
		for _, result := range results {
			if !result {
				return false
			}
		}
		return true
	} else { // OR logic
		for _, result := range results {
			if result {
				return true
			}
		}
		return false
	}
}

func sortEntries(entries []HAREntry, sortBy string) {
	sort.Slice(entries, func(i, j int) bool {
		switch sortBy {
		case "time":
			return entries[i].StartedDateTime.Before(entries[j].StartedDateTime)
		case "size":
			return entries[i].ResponseSize < entries[j].ResponseSize
		case "status":
			return entries[i].Status < entries[j].Status
		case "url":
			return entries[i].URL < entries[j].URL
		default:
			return false
		}
	})
}

func chunkEntries(entries []HAREntry, chunkSize int) [][]HAREntry {
	var chunks [][]HAREntry
	for i := 0; i < len(entries); i += chunkSize {
		end := i + chunkSize
		if end > len(entries) {
			end = len(entries)
		}
		chunks = append(chunks, entries[i:end])
	}
	return chunks
}

func analyzeSummary(chunk []HAREntry) map[string]interface{} {
	summary := make(map[string]interface{})
	summary["total_entries"] = len(chunk)

	var totalTime float64
	var totalSize int64
	statusCodes := make(map[int]int)

	for _, entry := range chunk {
		totalTime += entry.ResponseTime
		totalSize += entry.ResponseSize
		statusCodes[entry.Status]++
	}

	summary["total_time"] = totalTime
	summary["total_size"] = totalSize
	summary["status_codes"] = statusCodes

	return summary
}

func outputSummary(summary map[string]interface{}, chunkNum int, format string) {
	switch format {
	case "json":
		outputJSON(summary, chunkNum)
	case "csv":
		outputCSV(summary, chunkNum)
	default:
		outputText(summary, chunkNum)
	}
}

func outputJSON(summary map[string]interface{}, chunkNum int) {
	summary["chunk_number"] = chunkNum
	jsonData, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		log.Printf("Error marshaling JSON: %v", err)
		return
	}
	fmt.Println(string(jsonData))
}

func outputCSV(summary map[string]interface{}, chunkNum int) {
	writer := csv.NewWriter(os.Stdout)
	defer writer.Flush()

	headers := []string{"Chunk", "Total Entries", "Total Time", "Total Size"}
	writer.Write(headers)

	row := []string{
		strconv.Itoa(chunkNum),
		fmt.Sprintf("%d", summary["total_entries"]),
		fmt.Sprintf("%.2f", summary["total_time"]),
		fmt.Sprintf("%d", summary["total_size"]),
	}
	writer.Write(row)
}

func outputText(summary map[string]interface{}, chunkNum int) {
	fmt.Printf("Summary for Chunk %d:\n", chunkNum)
	fmt.Printf("  Total Entries: %d\n", summary["total_entries"])
	fmt.Printf("  Total Time: %.2f ms\n", summary["total_time"])
	fmt.Printf("  Total Size: %d bytes\n", summary["total_size"])
	fmt.Printf("  Status Code Distribution:\n")
	for code, count := range summary["status_codes"].(map[int]int) {
		fmt.Printf("    %d: %d\n", code, count)
	}
	fmt.Println()
}

func performAIAnalysis(chunk []HAREntry, apiKey string) (string, error) {
	ctx := context.Background()
	client, err := anthropic.New()
	if err != nil {
		return "", fmt.Errorf("error creating Anthropic client: %w", err)
	}

	chunkSummary := analyzeSummary(chunk)
	prompt := fmt.Sprintf("Analyze the following HAR file chunk summary and provide insights:\n%+v", chunkSummary)

	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, systemPrompt),
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	}

	resp, err := client.GenerateContent(ctx, messages, llms.WithTemperature(0.1), llms.WithMaxTokens(4000))
	if err != nil {
		return "", fmt.Errorf("error generating AI analysis: %w", err)
	}

	return resp.Choices[0].Content, nil
}
