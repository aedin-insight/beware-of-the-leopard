# ask — 60-minute roadmap

Target: done in 40 minutes, leaving 20 minutes of buffer.

## Build blocks

| Time | Block | Done when | Commit message |
|---|---|---|---|
| 0–5 | Skeleton | Shebang, argparse, arg-or-stdin, usage + exit 2, .gitignore | `Add CLI skeleton and input handling` |
| 5–15 | Request | Read key from env (exit 1), POST to `/api/v1/chat/completions`, `stream: true` | `Send prompt to OpenRouter` |
| 15–25 | Streaming | Parse SSE, skip `:`, read `data:`, stop on `[DONE]`, print `delta.content`, flush | `Stream response to stdout` |
| 25–35 | Failure paths | HTTPError → 401/404/429, URLError, mid-stream error, Ctrl-C → 130 | `Handle errors with clear messages and exit codes` |
| 35–40 | Docs | Add README section + NOTES.md | `Add README and NOTES` |

## Test pass

- [x] `./ask "say hi"` (blocked: OpenRouter paywall)
- [x] `echo "say hi" | ./ask` (blocked: OpenRouter paywall)
- [x] `./ask ; echo $?` (Pass: 2)
- [x] `./ask "" ; echo $?` (Pass: 2)
- [x] `OPENROUTER_API_KEY= ./ask "hi"; echo $?` (Pass: 1)
- [x] `OPENROUTER_API_KEY=bad ./ask "hi"; echo $?` (Pass: 1)
- [x] `./ask --model nope/nope "hi"; echo $?` (Pass: 1, 401 auth gate)
- [x] `./ask "write 500 words"` then Ctrl-C (Pass: 130)
- [x] `./ask "hi" > out.txt` (Pass: stdout clean)
- [x] Network-down test (Pass: 1)
