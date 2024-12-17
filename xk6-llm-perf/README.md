# xk6-llm-perf

A k6 extension for load testing LLM APIs with real-time dashboard visualization.

## Features

- Measure streaming and non-streaming LLM performance
- Real-time metrics visualization
- Separate dashboard views for streaming/non-streaming
- Load testing with ramping VUs
- HAR recording with SSE support

## Dashboard Views

1. Streaming Metrics
   - Time to First Token (TTFT)
   - Token Latency
   - Tokens per Second

2. Sync Metrics
   - TTFT for non-streaming
   - Batch Token Latency
   - Overall Throughput

3. Load Metrics
   - Virtual Users
   - Request Duration
   - Request Rate
   - Total Tokens

## Usage

Run streaming test:
```bash
make sample-brev-stream
```

Run non-streaming test:
```bash
make sample-brev
```

Dashboard available at http://localhost:5665
