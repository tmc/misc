/** Inference measurements for k6. Durations use milliseconds unless specified. */
declare module 'k6/x/llm-perf' {
  export interface Config {
    controlGroup?: string; apiKey?: string; baseURL?: string; baseURLs?: string[]; model?: string;
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
  export type Protocol = 'openai-chat' | 'openai-completions' | 'openai-responses' | 'anthropic' | 'tgi' | 'json' | 'audio-transcription' | 'image' | 'video' | 'retrieval' | 'openai-embeddings' | 'cohere-rankings' | 'nim-rankings' | 'hf-tei-rankings' | 'nim-image-retrieval' | 'nv-embedqa' | 'chat-embeddings' | 'nim-embeddings' | 'solido-rag';
  export interface RawRequest {
    path: string; protocol: Protocol; stream?: boolean; body?: Record<string, unknown>;
    multipart?: { fields?: Record<string,string>; files: {name:string; filename:string; contentType:string; base64:string}[] };
    poll?: {interval?:string; maxPolls?:number; download?:boolean};
    measurement?: TokenCounts; scheduledAt?: number; cancelAfter?: string;
  }
  export interface TokenCount { value: number | null; source: string; }
  export interface Measurement {
    choice_policy: string; multi_choice: boolean; provider_total_tokens: number | null; client_tokenizer?: string; request_id?: string; schema_version: 1; protocol: Protocol; phase: string;
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
    choices: { index: number; message: Message; finish_reason?: string }[];
    usage: { prompt_tokens: number; completion_tokens: number; total_tokens: number;
      reasoning_tokens?: number; cached_tokens?: number };
    sources?:unknown; body?: unknown; measurement?: Measurement;
  }
  export interface MeasurementError extends Error { measurement?: Measurement; }
  export interface RunOptions {
    runId?: string; workerId?: string; maxRecords?: number; maxRecordBytes?: number; traceHash?: string; seed?: number;
    workload?: string; startDelay?: string; startedAt?: number; duration?: string;
  }
  export interface Weighted { mean: number; p50: number; p95: number; p99: number; duration: number; }
  export interface Concurrency { effective: Weighted; active: Weighted; }
  export interface RateAnalysis {effective:Weighted; active:Weighted; eligible_requests:number; excluded_requests:number; policy:string}
  export interface RunSummary {
    retained_record_bytes: number; record_limit: number; record_byte_limit: number; worker_clocks?: Record<string, {offset_ms:number; uncertainty_ms:number; basis:string}>; worker_id: string; workers?: string[]; missing_workers?: string[]; schema_version: 1; run_id: string; started_at: number; finished_at: number; duration: number;
    attempts: number; successes: number; failures: number; good_requests: number;
    input_tokens: number; output_tokens: number; goodput: number | null; aggregate_error?:string;
    input_throughput: number | null; output_throughput: number | null;
    missing_input_counts: number; missing_output_counts: number;
    estimated_input_counts: number; estimated_output_counts: number;
    excluded_boundary_requests: number; dropped_records: number; inflight_at_end: number;
    timeline: null | { requests: Concurrency; prefill: Concurrency; decode: Concurrency;
      unsplit_requests: number; input_rate:RateAnalysis; output_rate:RateAnalysis; total_rate:RateAnalysis; phase_policy: string };
    timeline_unavailable?: string; trace_hash: string; seed: number; workload: string;
    window_policy: string; warmup_policy: string; versions: Record<string, string>;
    records: Measurement[];
  }
  export class Client {
    constructor(config: Config);
    chat: { completions: { create(request: CompletionRequest): CompletionResponse | MeasurementError } };
    request(request: RawRequest): CompletionResponse | MeasurementError;
  }
  export interface CalibrationOptions {
    url: string; count: number; timeout: string; interval?: string;
    method?: 'HEAD' | 'GET'; maxBodyBytes?: number; transportSource?: string;
  }
  export interface CalibrationReport {
    schemaVersion: 1; probeType: string; basis: string; options: CalibrationOptions;
    startedAt: number; duration: number; resolutionDuration: number | null;
    probes: {index:number; startedAt:number; duration:number; success:boolean; status:number;
      errorStage?:string; bytes:number; firstByte:number|null; rtt:number|null; trace:Measurement['http_trace']}[];
    successes:number; rttSamples:number; meanRTT:number|null; meanFirstByte:number|null;
    complete:boolean; runtime:Record<string,string>; clientTimeout:number|null;
  }
  export function calibrateHTTP(options: CalibrationOptions): CalibrationReport;
  export function calibrateTCP(options: CalibrationOptions): CalibrationReport;
  export function renderRequestTemplate(spec: {name?:string; body?:Record<string,unknown>; variables:Record<string,unknown>}):
    {body:Record<string,unknown>; provenance:{schemaVersion:1; scope:string; name?:string; templateSHA256:string; variablesSHA256:string; payloadSHA256:string}};
  export function setControlLimits(group:string, limits:{request:number; prefill:number}):void;
  export function controlSnapshot(group:string):{limits:{request:number; prefill:number}; requests:number; prefills:number};
  export function beginMeasurement(options: RunOptions): void;
  export function endMeasurement(): RunSummary;
  export function measurementSummary(): RunSummary;
  const llm: { Client: typeof Client; setControlLimits:typeof setControlLimits; controlSnapshot:typeof controlSnapshot; calibrateHTTP:typeof calibrateHTTP; calibrateTCP:typeof calibrateTCP; renderRequestTemplate:typeof renderRequestTemplate; beginMeasurement: typeof beginMeasurement;
    endMeasurement: typeof endMeasurement; measurementSummary: typeof measurementSummary };
  export default llm;
}
