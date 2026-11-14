# The CLI Challenge

**Aedin Insight, LLC** | Time box: 60 minutes | No calls, no decks

We hire on evidence, not resumes. This is a small, real task. If you know what you're doing it takes about 30 minutes.

## The task

Build a command-line tool that sends a prompt to a language model through [OpenRouter](https://openrouter.ai) and prints the response.

### Requirements

1. Run it as `ask "your prompt"`, or pipe the prompt in on stdin.
2. A `--model` flag selects the model. Default to any model with a `:free` suffix, so this costs nothing to run.
3. Stream the response to the terminal as it arrives.
4. Exit with a non-zero status when something goes wrong.

Use any language you like.

### Not required

A config system, plugins, a TUI, chat history, or a logo. We value small. One file is a fine answer.

## Rules

- **Questions go in GitHub Issues, in writing.** Answers are posted publicly so every candidate has the same information. Requests for a call get no reply. That is not rudeness, it is the policy, and it is how we work with clients too.
- **AI tools are allowed.** You will use them on the job, so use them here. You own whatever you submit.
- **Never commit or paste credentials anywhere**, including issues, commit messages, and screenshots.
- **Everything here is synthetic.** Nothing in this repo is real client work or real data, and we will never ask you to do real work for free.
- **60 minutes.** We read commit timestamps, so there is no honor system to maintain. If you went over, say so in your notes.

## What to submit

1. Fork this repo and create a branch named after you.
2. Commit your solution to that branch.
3. Include a `NOTES.md` of 10 lines or fewer: what you built, what you cut, what you assumed, and what you would ask us if you could.
4. Open a pull request with a clear description.

Pull requests are public, so keep anything private out of them.

## What we look at

- How it behaves when things go wrong
- How it handles secrets
- Whether it feels finished at the terminal, not just correct on paper
- What you chose not to build
- How you use git and GitHub: branch, commits, pull request
- Your `NOTES.md`

After we review, you get the full rubric in writing. If you missed something and fix it, send the fix. A fast, clean follow-up counts for a lot.

## No time for a take-home?

Open an issue titled `Existing work: <your name>` with a link to something you are proud of, and a short note on what we should look at and why. We will treat it as a submission.

## Reference

- OpenRouter docs: https://openrouter.ai/docs

---

*Aedin Insight, LLC | Albuquerque, NM | aedininsight.com*

## Implementation (James Gilbert)

Built as a single-file Python 3 script, standard library only. 
Streams responses via SSE, strictly reads `OPENROUTER_API_KEY` from the environment, and routes all errors to `stderr`.

See `NOTES.md` for known limitations (OpenRouter paywall blocked happy-path testing).
See `CHANGELOG.md` for the build history.

### Test Results

| Case | Result | Exit Code |
|---|---|---|
| `./ask ; echo $?` | Usage printed | 2 |
| `./ask "" ; echo $?` | Usage printed | 2 |
| `OPENROUTER_API_KEY= ./ask "hi"` | Missing key | 1 |
| `OPENROUTER_API_KEY=bad ./ask "hi"` | Auth failed (401) | 1 |
| `./ask --model nope/nope "hi"` | Model not found (blocked by 401) | 1 |
| `Ctrl-C` during stream | Clean exit | 130 |
| `Network disconnected` | Network error | 1 |
