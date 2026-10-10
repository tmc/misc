/** Inference measurements for k6. Durations use milliseconds unless specified. */
declare module 'k6/x/llm-perf' {
  export interface Config {
    apiKey?: string; baseURL?: string; baseURLs?: string[]; model?: string;
    timeout?: string; networkRTT?: number; rttSource?: string;
    tokenMultiplier?: number; prefillConcurrency?: number;
    requestConcurrency?: number; useK6Transport?: boolean; warmupCount?: number;
    maxTTFT?: number; maxTTFO?: number; maxTokenLatency?: number;
    strictMeasurement?: boolean; discrepancyTolerance?: number;
    phase?: 'warmup' | 'measurement' | 'cooldown';
  }
  export interface ToolCall {
    index?: number; id?: string; type?: string;
    function: { name: string; arguments: string };
  }
  export interface Message {
    role: string; content: string | Record<string, unknown>[] | null;
    reasoning_content?: string; tool_calls?: ToolCall[];
    tool_call_id?: string; name?: string;
  }
  /** These counts are caller declarations, not independently verified counts. */
  export interface TokenCounts {
    inputTokens?: number; outputTokens?: number; firstChunkTokens?: number;
    tokenizer: string;
  }
  export interface CompletionRequest {
    model?: string; messages: Message[]; temperature?: number;
    max_tokens?: number; max_completion_tokens?: number; stream?: boolean;
    stream_options?: { include_usage?: boolean; continuous_usage_stats?: boolean };
    tools?: Record<string, unknown>[]; tool_choice?: unknown;
    measurement?: TokenCounts; scheduledAt?: number; cancelAfter?: string;
  }
  export type Protocol = 'openai-chat' | 'openai-completions' | 'openai-responses' | 'anthropic' | 'json';
  export interface RawRequest {
    path: string; protocol: Protocol; body: Record<string, unknown>;
    measurement?: TokenCounts; scheduledAt?: number; cancelAfter?: string;
  }
  export interface TokenCount { value: number | null; source: string; }
  export interface Measurement {
    provider_total_tokens: number | null; client_tokenizer?: string; request_id?: string; schema_version: 1; protocol: Protocol; phase: string;
    endpoint: string; model: string; outcome: string; error_stage?: string;
    status: number; attempted: boolean; recorded: boolean; good: boolean;
    started_at: number; dispatched_at: number; finished_at: number;
    scheduled_at: number | null; arrival_lag: number | null;
    scheduled_response_latency: number | null;
    admission_wait: number; full_response_latency: number;
    last_content_latency: number | null; ttft: number | null; ttfo: number | null;
    max_chunk_latency: number | null; itl: number | null;
    full_response_itl: number | null; decode_tokens: number | null;
    first_chunk_source: string; input_tokens: TokenCount; output_tokens: TokenCount;
    rtt: number; rtt_source: string; usage_discrepancy: boolean;
    prompt_difference: number | null; output_difference: number | null;
    reasoning_tokens: number | null; cached_tokens: number | null;
    http_trace: null | { dns: number | null; connect: number | null; tls: number | null;
      sending: number | null; waiting: number | null; receiving: number | null; reused: boolean | null };
    spec_decode: null | { engine: string; accepted: number; drafted: number; steps: number;
      histogram: number[]; acceptance_rate: number | null; mean_acceptance_length: number | null };
    spec_decode_unavailable?: string;
  }
  export interface CompletionResponse {
    id: string; object: string; created: number; model: string; status: number;
    choices: { message: Message; finish_reason?: string }[];
    usage: { prompt_tokens: number; completion_tokens: number; total_tokens: number;
      reasoning_tokens?: number; cached_tokens?: number };
    body?: unknown; measurement?: Measurement;
  }
  export interface MeasurementError extends Error { measurement?: Measurement; }
  export interface RunOptions {
    runId?: string; workerId?: string; maxRecords?: number; maxRecordBytes?: number; traceHash?: string; seed?: number;
    workload?: string; startDelay?: string; duration?: string;
  }
  export interface Weighted { mean: number; p50: number; p95: number; p99: number; duration: number; }
  export interface Concurrency { effective: Weighted; active: Weighted; }
  export interface RunSummary {
    retained_record_bytes: number; record_limit: number; record_byte_limit: number; worker_clocks?: Record<string, {offset_ms:number; uncertainty_ms:number; basis:string}>; worker_id: string; workers?: string[]; missing_workers?: string[]; schema_version: 1; run_id: string; started_at: number; finished_at: number; duration: number;
    attempts: number; successes: number; failures: number; good_requests: number;
    input_tokens: number; output_tokens: number; goodput: number;
    input_throughput: number; output_throughput: number;
    missing_input_counts: number; missing_output_counts: number;
    estimated_input_counts: number; estimated_output_counts: number;
    excluded_boundary_requests: number; dropped_records: number; inflight_at_end: number;
    timeline: null | { requests: Concurrency; prefill: Concurrency; decode: Concurrency;
      unsplit_requests: number; phase_policy: string };
    timeline_unavailable?: string; trace_hash: string; seed: number; workload: string;
    window_policy: string; warmup_policy: string; versions: Record<string, string>;
    records: Measurement[];
  }
  export class Client {
    constructor(config: Config);
    chat: { completions: { create(request: CompletionRequest): CompletionResponse | MeasurementError } };
    request(request: RawRequest): CompletionResponse | MeasurementError;
  }
  export function beginMeasurement(options: RunOptions): void;
  export function endMeasurement(): RunSummary;
  export function measurementSummary(): RunSummary;
  const llm: { Client: typeof Client; beginMeasurement: typeof beginMeasurement;
    endMeasurement: typeof endMeasurement; measurementSummary: typeof measurementSummary };
  export default llm;
}
