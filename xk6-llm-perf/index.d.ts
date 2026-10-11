/** Inference measurements for k6. Durations use milliseconds unless specified. */
declare module 'k6/x/llm-perf' {
  /** Synchronous JSON-only evaluation; cost is outside HTTP timing. */
  export function evaluateRuntimeTemplate(options: {
    python: string; pythonSHA256: string; script: string; scriptSHA256: string;
    request: Record<string, unknown>;
  }): { result: { schema_version: number; operation: string; value: unknown; provenance: Record<string, unknown> }; elapsed_ms: number; script_sha256: string } | Error;
  export interface Config {
    controlGroup?: string; apiKey?: string; baseURL?: string; baseURLs?: string[]; model?: string;
    maxConnsPerHost?: number; timeout?: string; networkRTT?: number; rttSource?: string;
    tokenMultiplier?: number; prefillConcurrency?: number;
    requestConcurrency?: number; useK6Transport?: boolean; warmupCount?: number;
    maxTTFT?: number; maxTTFO?: number; maxTokenLatency?: number; maxTPOT?: number; maxE2E?: number;
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
  export interface WorkExpectation {
    allowedFinishReasons?: string[]; minOutputTokens?: number; maxOutputTokens?: number;
    expectedModel?: string; requireExactUsage?: boolean; textDigest?: string;
    responseDigest?: string; tokenIDsDigest?: string;
  }
  export interface WorkReceipt {
    expectation: WorkExpectation; status: 'accepted' | 'rejected' | 'unavailable';
    reasons: string[] | null; observed_model?: string; finish_reasons?: string[];
    exact_usage: boolean; reported_output_tokens: number | null;
    text_digest?: string; response_digest?: string; token_ids_digest?: string;
  }
  export interface CompletionRequest {
    model?: string; messages: Message[]; temperature?: number;
    max_tokens?: number; max_completion_tokens?: number; stream?: boolean;
    stream_options?: { include_usage?: boolean; continuous_usage_stats?: boolean };
    tools?: Record<string, unknown>[]; tool_choice?: unknown;
    work?: WorkExpectation; measurement?: TokenCounts; scheduledAt?: number; cancelAfter?: string;
  }
  export type Protocol = 'openai-chat' | 'openai-completions' | 'openai-responses' | 'anthropic' | 'tgi' | 'json' | 'audio-transcription' | 'image' | 'video' | 'retrieval' | 'openai-embeddings' | 'cohere-rankings' | 'nim-rankings' | 'hf-tei-rankings' | 'nim-image-retrieval' | 'nv-embedqa' | 'chat-embeddings' | 'nim-embeddings' | 'solido-rag';
  export interface RawRequest {
    path: string; protocol: Protocol; stream?: boolean; body?: Record<string, unknown>;
    multipart?: { fields?: Record<string,string>; files: {name:string; filename:string; contentType:string; base64:string}[] };
    poll?: {interval?:string; maxPolls?:number; download?:boolean};
    work?: WorkExpectation; measurement?: TokenCounts; scheduledAt?: number; cancelAfter?: string;
  }
  export interface TokenCount { value: number | null; source: string; }
  export interface Measurement {
    request_body_bytes?: number; response_body_bytes?: number; max_connections_per_host?: number; transport_profile?: string; stream?: boolean;
    serialization_latency_ms?: number; parsing_latency_ms?: number;
    work?: WorkReceipt;
    choice_policy: string; multi_choice: boolean; provider_total_tokens: number | null; client_tokenizer?: string; request_id?: string; schema_version: 1; protocol: Protocol; phase: string;
    endpoint: string; model: string; outcome: string; error_stage?: string;
    status: number; attempted: boolean; recorded: boolean; good: boolean;
    started_at: number; dispatched_at: number; finished_at: number; timestamp_basis?: string;
    scheduled_at: number | null; arrival_lag: number | null;
    scheduled_response_latency: number | null;
    admission_wait: number; full_response_latency: number;
    last_content_latency: number | null; ttft: number | null; ttfo: number | null;
    max_chunk_latency: number | null; itl: number | null;
    full_response_itl: number | null; decode_tokens: number | null;
    content_intervals_ms: number[] | null; content_intervals_truncated: boolean;
    content_event_count: number; ttst: number | null; ttst_policy?: string;
    slo?: {max_ttft_ms: number; max_ttfo_ms: number; max_chunk_latency_ms: number; max_tpot_ms?: number; max_e2e_ms?: number};
    first_chunk_source: string; first_chunk_tokens?: number; first_chunk_origin?: string;
    first_chunk_unavailable?: string; input_tokens: TokenCount; output_tokens: TokenCount;
    rtt: number; rtt_source: string; usage_discrepancy: boolean;
    prompt_difference: number | null; output_difference: number | null;
    reasoning_tokens: number | null; cached_tokens: number | null;
    http_trace: null | { dns: number | null; connect: number | null; tls: number | null;
      sending: number | null; waiting: number | null; receiving: number | null; reused: boolean | null };
    spec_decode: null | { engine: string; source?: string; choice_index?: number; accepted: number; drafted: number; steps: number;
      histogram: number[]; acceptance_rate: number | null; mean_acceptance_length: number | null };
    spec_decode_unavailable?: string;
  }
  export interface CompletionResponse {
    id: string; object: string; created: number; model: string; status: number; provider_status?: string;
    choices: { index: number; message: Message; finish_reason?: string }[];
    usage: { prompt_tokens: number; completion_tokens: number; total_tokens: number;
      reasoning_tokens?: number; cached_tokens?: number };
    sources?:unknown; body?: unknown; measurement?: Measurement;
  }
  export interface MeasurementError extends Error { measurement?: Measurement; }
  export interface ServingSetting { configured?: unknown; effective?: unknown; evidence_digest?: string; }
  export interface ServingManifest {
    schema_version: 1;
    arm: { binary_digest?: string; source_revision?: string; backend?: string; endpoint_identity?: string };
    model_digest?: string; tokenizer_digest?: string; template_digest?: string;
    workload_digest?: string; environment_digest?: string;
    settings?: Partial<Record<'cache' | 'compile' | 'device' | 'dtype' | 'max_output_tokens' | 'resources' | 'scheduler' | 'seed' | 'temperature' | 'top_k' | 'top_p', ServingSetting>>;
    confounds?: string[];
  }
  export interface RunOptions {
    servingManifest?: ServingManifest;
    runId?: string; workerId?: string; maxRecords?: number; maxRecordBytes?: number; traceHash?: string; seed?: number;
    workload?: string; startDelay?: string; startedAt?: number; duration?: string;
  }
  export interface Weighted { mean: number; p50: number; p95: number; p99: number; duration: number; }
  export interface Concurrency { effective: Weighted; active: Weighted; }
  export interface RateAnalysis {effective:Weighted; active:Weighted; eligible_requests:number; excluded_requests:number; policy:string}
  export interface RunSummary {
    observation?: {policy: string; started_at: number; finished_at: number; duration: number; attempts: number; good_requests: number; goodput: number};
    observation_unavailable?: string;
    work_qualified?: number; work_accepted?: number; work_rejected?: number;
    work_unavailable?: number; work_good_requests?: number; work_goodput?: number;
    serving_manifest?: ServingManifest;
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
    measurement_policy?: string; window_policy: string; warmup_policy: string; versions: Record<string, string>;
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
  /** Per-VU sample delivery receipt; exact only after producers stop. */
  export interface PipelineSnapshot {
    generation: number; attempted: number; delivered: number; cancelled: number;
    wait_nanoseconds: number; max_wait_nanoseconds: number; inflight: number;
  }
  export function pipelineSnapshot(): PipelineSnapshot;
  /** Requires no active producers; starts a new per-VU receipt generation. */
  export function resetPipeline(): void;
  export function beginMeasurement(options: RunOptions): void;
  export function endMeasurement(): RunSummary;
  export function measurementSummary(): RunSummary;
  const llm: { evaluateRuntimeTemplate: typeof evaluateRuntimeTemplate; Client: typeof Client; setControlLimits:typeof setControlLimits; controlSnapshot:typeof controlSnapshot; calibrateHTTP:typeof calibrateHTTP; calibrateTCP:typeof calibrateTCP; renderRequestTemplate:typeof renderRequestTemplate; beginMeasurement: typeof beginMeasurement;
    endMeasurement: typeof endMeasurement; measurementSummary: typeof measurementSummary };
  export default llm;
}
