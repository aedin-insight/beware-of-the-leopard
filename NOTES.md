Built: single-file Python CLI, stdlib only, streams via SSE.
Secrets: env var only; no key flag (keeps it out of shell history/ps).
Errors: stderr only; exit 0/1/2/130 documented in README.
Cut: retries, config, history, colour, model listing.
Assumed: Python 3.8+ available; mid-stream errors arrive as an `error` object.
Blocker: OpenRouter UI requires setting a credit limit ($10) before issuing a key (easily misread as a hard paywall).
Tested: Usage (2), Missing Key (1), Auth 401 (1), Network (1), Happy Path (0).
Unable to test initially: Happy path was blocked by UI confusion on billing.
Ask: Is the contract route open to UK sole traders? 

## Decisions

**Retry on 429:** I chose fail fast (exit 1). For a CLI designed to be piped into other tools (`echo "..." | ask > out.txt`), silent retries complicate the contract. If a 429 occurs, I want the shell script to know immediately so it can decide whether to sleep or alert the operator. Exponential backoff is a great addition for a v2, but for a 60-minute constraint, predictable failure is better than unpredictable latency.

---

## Stage 2: Scaling to 500 RPM

**What breaks first?**
1. **Provider Rate Limits (429s):** A fleet of agents blasting 500 requests/minute will hit OpenRouter or underlying model limits immediately. 
2. **Process Spawning:** Forking 500 Python processes a minute (`./ask`) is a massive waste of CPU and memory. The OS will thrash before the network breaks.
3. **Observability & Retries:** With no centralised queue or logging, a dropped connection or a mid-stream error vanishes into an agent's stdout. There is no retry mechanism; a failed call is just a failed agent.

**Build vs. Buy:**
- **Buy (or adopt OSS):** A long-running API gateway with built-in rate limiting and fallback (e.g., LiteLLM or a lightweight FastAPI + Redis token bucket). We must buy the connection pooling, retry logic, and provider-failover. 
- **Build:** The agent orchestration layer. We need a thin, unified client library that knows *which* model to route to (e.g., cheap/fast for parsing, expensive/smart for reasoning). 

**What I refuse to build:**
- **The CLI:** I would refuse to scale this script. The architecture must shift from a spawned binary to a long-running HTTP/gRPC service.
- **A custom provider wrapper:** I would refuse to hand-roll retry/backoff logic for 10 different LLM APIs when LiteLLM or a Redis-backed queue does it better and safer.
- **A billing/token manager:** If agents are spending money, let the provider (OpenRouter) handle the billing constraints.

**One question for the client before starting:**
`"What is the blast radius of a single failed call? If a model provider goes down mid-stream, does the agent fail the whole task, or can it retry with a fallback model without losing state?"`

    
## Model Resilience
OpenRouter frequently retires `:free` models. The CLI now includes a `FALLBACK_MODELS` list. It attempts the specified model (or `openrouter/free` as a router default), and if it hits a 404 (model retired) or 429 (rate limit), it seamlessly falls back to the next available free model, logging the attempt to stderr so the stdout stream remains clean. This directly addresses the provider-deprecation failure mode discussed in Stage 2.
