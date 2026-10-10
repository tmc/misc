/** OpenAI-compatible completions for k6. Latency options use milliseconds. */
declare module 'k6/x/llm-perf' {
  export interface Config {
    apiKey?: string;
    baseURL?: string;
    baseURLs?: string[];
    model?: string;
    timeout?: string;
    networkRTT?: number;
    tokenMultiplier?: number;
    prefillConcurrency?: number;
    warmupCount?: number;
    maxTTFT?: number;
    maxTTFO?: number;
    maxTokenLatency?: number;
  }
  export interface Message { role: string; content: string; }
  export interface CompletionRequest {
    model?: string;
    messages: Message[];
    temperature?: number;
    max_tokens?: number;
    stream?: boolean;
    stream_options?: { include_usage?: boolean };
  }
  export interface CompletionResponse {
    id: string;
    object: string;
    created: number;
    model: string;
    status: number;
    choices: { message: Message; finish_reason?: string }[];
    usage: { prompt_tokens: number; completion_tokens: number; total_tokens: number };
  }
  export class Client {
    constructor(config: Config);
    chat: { completions: { create(request: CompletionRequest): CompletionResponse | Error } };
  }
  const llm: { Client: typeof Client };
  export default llm;
}
