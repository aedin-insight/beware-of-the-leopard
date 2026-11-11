# Notes

- **Built:** `ask`, one stdlib-only Go file. `go build`, then `ask "prompt"` or `echo prompt | ask`; `-m/--model` works before or after the prompt; default `google/gemma-4-31b-it:free`.
- **When things go wrong:** messages go to stderr with exit 1 (request or stream failed), 2 (usage error, missing key) or 130 (Ctrl-C). Covers HTTP errors with a hint (401, 402, 429, unknown model), errors sent mid-stream, dropped connections, 90s of silence and empty replies. Output always ends on a newline, so an error never lands mid-line.
- **Secrets:** the key comes only from `OPENROUTER_API_KEY`. There is no `--key` flag, so it never lands in shell history or `ps`; it is never printed; `.env` is gitignored.
- **Tests:** `go test` runs against a fake OpenRouter (`httptest`), so it needs no key and no network.
- **Cut:** combining an argument with piped input (`cat log | ask "explain"`), because reading stdin you might not need can hang in scripts; retries; system prompts; temperature and other parameters; showing reasoning tokens.
- **Assumed:** a prompt argument wins over stdin. Free model IDs rotate, so the default may need bumping (it was live on OpenRouter's model list today).
- **Would ask:** on a 429 from a free model, should `ask` retry with backoff or fail fast as it does now?
