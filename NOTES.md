
**Built:** `ask`, a single-file Python 3 CLI (stdlib only, nothing to install). Prompt from args or stdin, `-m/--model` (default `nvidia/nemotron-3.5-lightning:free`), streams tokens via SSE, `ask: ...` errors on stderr.
**Exit codes:** 0 ok · 1 missing key / HTTP / network / timeout / mid-stream or empty reply · 2 no prompt · 130 Ctrl+C. Quiet exit when piped into `head`.
**Secrets:** key only from `OPENROUTER_API_KEY` env var; never logged or echoed. `.env` is gitignored (first commit) for local use: `set -a; . ./.env; set +a`.
**Cut:** config files, .env auto-loading, retries/backoff, system prompts, chat history, packaging (`pip install`). Copy or symlink `ask` onto your PATH.
**Assumed:** the user exports the key themselves; one-shot prompts only; 60s network timeout is fine.
**Gotcha found:** the first free default (`google/gemma-4-31b-it:free`) returned 429 in testing, so free defaults are fragile. 429s now print a hint to use `--model`.
**Tested:** arg, stdin, `--model`, invalid model (400), bad key (401), missing key, no prompt, rate limit (429), `| head`, streaming timing.
**Would ask:** should the default track whatever free model is up (e.g. `openrouter/auto`), and do you want automatic retry on 429?
**Time:** about 10 minutes from fork to PR (see commit timestamps).

---

## Stage two: ask at 500 calls/min from a fleet of agents

**What breaks first**

Rate limits. 500 a minute is about 8 requests per second and right now everything goes through one key and a free model. I already got 429s while testing with just a few calls, so at this volume it would fail almost straight away. Free tier can't take this volume at all, so it has to move to a paid tier, which is the client's call. And if a call takes around 10 seconds that's roughly 80 calls in flight at once, which is why the concurrency limits matter.

The bigger problem is what happens after the 429. Right now ask exits with 1 for everything, so an agent can't tell "rate limited, try again in a bit" apart from "bad model, never going to work". So agents either give up on errors that would have passed on a retry, or they keep retrying stuff that will never work. And if 50 agents all retry at the same moment you just make the rate limiting worse.

After that:
- if one provider goes down, every agent goes down with it, there's no failover
- no cost control. One shared key, no budget per agent, no way to see which agent spent what
- streaming text to a terminal makes sense for a person, not for an agent that has to parse the output

**What I would buy**

First I'd use what OpenRouter already gives us: a paid tier, one key per agent with a credit limit, and an approved model list. If we need providers OpenRouter doesn't have, or the client needs data to stay in their network, then an LLM gateway like LiteLLM proxy or Portkey in front. These already do the hard parts: a separate key per agent, rate limits and budgets, retries with backoff and jitter, failover between providers, and logging of requests and cost. Building all that ourselves would take weeks and we'd probably get it wrong.

**What I would build**

Only the small stuff the gateway can't do for us:
- ask can stay as the thing agents call, it just points at the gateway with one env var instead of OpenRouter. For that it needs:
- proper exit codes: 75 (EX_TEMPFAIL) for retryable stuff like 429, 5xx and timeouts, and 1 for permanent errors like a bad key or a bad model. Then agents know when to retry
- a `--json` flag that returns the text, the model used and token usage, and respecting the `Retry-After` header
- a concurrency limit per agent, so one runaway agent can't eat the whole quota

**What I wouldn't build**

- our own router, load balancer or multi-provider SDK. That's already solved and it's a lot to maintain
- silent fallback to paid models. Same as your point on stage one: failover only to models the client approved, inside the budget
- retry forever, or our own queue system, until we've actually measured that we need it
- caching, chat history or a custom dashboard. Nobody asked for them yet and the gateway logs cover most of it at the start

**Question I'd ask the client first**

When a provider fails or the budget runs out, do you want the call to just fail, or fall back to another model that might cost more?
