
**Built:** `ask`, a single-file Python 3 CLI (stdlib only, nothing to install). Prompt from args or stdin, `-m/--model` (default `nvidia/nemotron-3.5-lightning:free`), streams tokens via SSE, `ask: ...` errors on stderr.
**Exit codes:** 0 ok · 1 missing key / HTTP / network / timeout / mid-stream or empty reply · 2 no prompt · 130 Ctrl+C. Quiet exit when piped into `head`.
**Secrets:** key only from `OPENROUTER_API_KEY` env var; never logged or echoed. `.env` is gitignored (first commit) for local use: `set -a; . ./.env; set +a`.
**Cut:** config files, .env auto-loading, retries/backoff, system prompts, chat history, packaging (`pip install`). Copy or symlink `ask` onto your PATH.
**Assumed:** the user exports the key themselves; one-shot prompts only; 60s network timeout is fine.
**Gotcha found:** the first free default (`google/gemma-4-31b-it:free`) returned 429 in testing, so free defaults are fragile. 429s now print a hint to use `--model`.
**Tested:** arg, stdin, `--model`, invalid model (400), bad key (401), missing key, no prompt, rate limit (429), `| head`, streaming timing.
**Would ask:** should the default track whatever free model is up (e.g. `openrouter/auto`), and do you want automatic retry on 429?
**Time:** about 10 minutes from fork to PR (see commit timestamps).
