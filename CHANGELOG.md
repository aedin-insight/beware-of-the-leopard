# CHANGELOG — ask

## [v1.1.1] — Resilience update

### Good
- Added `FALLBACK_MODELS` list to `ask`.
- If a model is retired (404) or rate-limited (429), the CLI automatically tries the next model in the list.
- Logs the model attempt to stderr, keeping stdout clean for piping.
- Verified happy path with `openrouter/free`.

### Bad
- Hardcoding fallback models still requires manual updates when OpenRouter retires the entire list.

### Ugly
- Discovered that `meta-llama/llama-3.3-70b-instruct:free` was retired mid-interview. The router (`openrouter/free`) is the most stable default.

## [v1.1.0] — Post-submission fixes (review feedback)

### Good
- Verified the happy path. Successfully streamed a reply from OpenRouter
  using a `:free` model (exit 0).
- Updated default model from `meta-llama/llama-3.2-3b-instruct:free` (retired)
  to `meta-llama/llama-3.3-70b-instruct:free`.
- Updated `NOTES.md` with:
  - The 429 fail-fast design rationale.
  - The Stage 2 (500 RPM) architecture answer (build vs. buy, refused scope).
  - Reworded the OpenRouter blocker to accurately reflect the billing UI.

### Bad
- The original default model was retired by OpenRouter, causing a
  `model not found` error on the first real happy-path test.
- Initially assumed the OpenRouter key generation required a hard
  paywall, which was incorrect.

### Ugly
- The OpenRouter UI requires setting a "Credit limit" (we used $10)
  before issuing a key. This looks exactly like requiring a credit
  card upfront and caused the wrong assumption in the original notes.
  Lesson: test the happy path before claiming a blocker, and check
  the actual UI carefully.

## [v1.0.0] — Challenge Submission

### Good
- Built single-file Python CLI (`ask`), standard library only.
- Implemented strict SSE parsing, flushing `delta.content` live to stdout.
- Read API key exclusively from `OPENROUTER_API_KEY` environment variable.
- Error handling: mapped `HTTPError` and `URLError` cleanly to stderr.
- Graceful `KeyboardInterrupt` handling (exit 130, no traceback).
- Committed core logic, `NOTES.md`, and executable bit (commit 79180e2).

### Bad
- Could not test the 200 OK happy path (streaming) because I initially
  believed OpenRouter required a credit card to generate an API key.

### Ugly
- Tried to update PR body with `gh pr edit --body "..."`, but the shell
  executed the markdown backticks as commands. Terminal spam ensued.
