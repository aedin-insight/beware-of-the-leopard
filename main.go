// Command ask sends a prompt to a language model through OpenRouter and
// streams the reply to stdout.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"
)

const defaultModel = "google/gemma-4-31b-it:free"

// Exit codes.
const (
	exitOK          = 0
	exitFailure     = 1   // the request or the stream failed
	exitUsage       = 2   // bad flags, no prompt, or no API key
	exitInterrupted = 130 // Ctrl-C
)

const usageText = `Usage: ask [--model MODEL] "prompt"
       echo "prompt" | ask [--model MODEL]

Sends a prompt to a language model through OpenRouter and streams the reply.

Options:
  -m, --model MODEL   model to use (default %s)
  -h, --help          show this help

Environment:
  OPENROUTER_API_KEY  required; create one at https://openrouter.ai/keys

Exit status: 0 ok, 1 request failed, 2 usage error, 130 interrupted.
`

var (
	endpoint    = "https://openrouter.ai/api/v1/chat/completions"
	idleTimeout = 90 * time.Second // give up if OpenRouter sends nothing for this long

	client = &http.Client{Transport: &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DialContext:         (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
		ForceAttemptHTTP2:   true,
	}}

	errHelp    = errors.New("help requested")
	errStalled = errors.New("stalled")
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := run(ctx, os.Args[1:], os.Stdin, stdinIsPiped(), os.Stdout, os.Stderr, os.Getenv)
	stop()
	os.Exit(code)
}

func stdinIsPiped() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice == 0
}

func run(ctx context.Context, args []string, stdin io.Reader, piped bool, stdout, stderr io.Writer, getenv func(string) string) int {
	model, prompt, err := parseArgs(args)
	if errors.Is(err, errHelp) {
		fmt.Fprintf(stdout, usageText, defaultModel)
		return exitOK
	}
	if err != nil {
		fmt.Fprintf(stderr, "ask: %v\nRun 'ask --help' for usage.\n", err)
		return exitUsage
	}

	if prompt == "" {
		if !piped {
			fmt.Fprintf(stderr, usageText, defaultModel)
			return exitUsage
		}
		b, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "ask: reading stdin: %v\n", err)
			return exitFailure
		}
		prompt = string(b)
	}
	if strings.TrimSpace(prompt) == "" {
		fmt.Fprintln(stderr, "ask: the prompt is empty")
		return exitUsage
	}

	key := strings.TrimSpace(getenv("OPENROUTER_API_KEY"))
	if key == "" {
		fmt.Fprintln(stderr, "ask: OPENROUTER_API_KEY is not set.\nCreate a key at https://openrouter.ai/keys and export it in your shell.")
		return exitUsage
	}

	err = stream(ctx, key, model, prompt, stdout, stderr)
	switch {
	case err == nil:
		return exitOK
	case ctx.Err() != nil:
		fmt.Fprintln(stderr, "ask: interrupted")
		return exitInterrupted
	default:
		fmt.Fprintf(stderr, "ask: %v\n", err)
		return exitFailure
	}
}

// parseArgs accepts flags before or after the prompt words, which are joined
// with spaces so `ask what is a leopard` works without quotes.
func parseArgs(args []string) (model, prompt string, err error) {
	model = defaultModel
	var words []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			words = append(words, args[i+1:]...)
			i = len(args)
		case a == "-h" || a == "--help":
			return "", "", errHelp
		case a == "-m" || a == "--model":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("%s needs a model name", a)
			}
			i++
			model = args[i]
		case strings.HasPrefix(a, "--model="):
			model = strings.TrimPrefix(a, "--model=")
		case len(a) > 1 && strings.HasPrefix(a, "-"):
			return "", "", fmt.Errorf("unknown flag %q (put -- before a prompt that starts with -)", a)
		default:
			words = append(words, a)
		}
	}
	if strings.TrimSpace(model) == "" {
		return "", "", errors.New("--model needs a model name")
	}
	return model, strings.Join(words, " "), nil
}

type apiError struct {
	Message string `json:"message"`
}

type chunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *apiError `json:"error"`
}

func stream(parent context.Context, key, model, prompt string, stdout, stderr io.Writer) error {
	ctx, cancel := context.WithCancelCause(parent)
	defer cancel(nil)

	body, err := json.Marshal(map[string]any{
		"model":    model,
		"stream":   true,
		"messages": []map[string]string{{"role": "user", "content": prompt}},
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("X-Title", "ask")

	// One timer covers both the wait for headers and silence between chunks.
	timer := time.AfterFunc(idleTimeout, func() { cancel(errStalled) })
	defer timer.Stop()

	resp, err := client.Do(req)
	if err != nil {
		return netError(ctx, "could not reach OpenRouter", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return statusError(resp)
	}

	out := &trackingWriter{w: stdout}
	// Never leave the cursor mid-line, whether we finish or fail.
	defer func() {
		if out.n > 0 && out.last != '\n' {
			io.WriteString(stdout, "\n")
		}
	}()

	var (
		r        = bufio.NewReader(resp.Body)
		finish   string // finish_reason from the last choice that set one
		complete bool   // saw the [DONE] marker
	)
	for {
		line, readErr := r.ReadString('\n')
		timer.Reset(idleTimeout)

		// SSE: lines starting with ':' are keep-alive comments; only data lines matter.
		line = strings.TrimRight(line, "\r\n")
		if data, ok := strings.CutPrefix(line, "data:"); ok {
			data = strings.TrimSpace(data)
			if data == "[DONE]" {
				complete = true
				break
			}
			var c chunk
			if err := json.Unmarshal([]byte(data), &c); err != nil {
				return fmt.Errorf("unreadable data in the response stream: %w", err)
			}
			if c.Error != nil {
				return fmt.Errorf("the model failed mid-response: %s", c.Error.Message)
			}
			for _, ch := range c.Choices {
				if _, err := io.WriteString(out, ch.Delta.Content); err != nil {
					return fmt.Errorf("writing output: %w", err)
				}
				if ch.FinishReason != "" {
					finish = ch.FinishReason
				}
			}
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return netError(ctx, "connection lost mid-response", readErr)
		}
	}

	if !complete && finish == "" {
		return errors.New("the response stream ended before the model finished")
	}
	if out.n == 0 {
		return errors.New("the model returned an empty response; try again or pick another --model")
	}
	switch finish {
	case "length":
		fmt.Fprintln(stderr, "ask: warning: the response was cut off at the model's length limit")
	case "content_filter":
		fmt.Fprintln(stderr, "ask: warning: the provider's content filter stopped the response")
	case "error":
		return errors.New("the model reported an error and stopped")
	}
	return nil
}

// netError explains why a network operation failed, distinguishing our own
// idle timeout from a Ctrl-C (which run reports) and from real network errors.
func netError(ctx context.Context, what string, err error) error {
	if errors.Is(context.Cause(ctx), errStalled) {
		return fmt.Errorf("%s: no data from OpenRouter for %s, giving up", what, idleTimeout)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return fmt.Errorf("%s: %w", what, err)
}

func statusError(resp *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	var body struct {
		Error apiError `json:"error"`
	}
	msg := strings.TrimSpace(string(raw))
	if json.Unmarshal(raw, &body) == nil && body.Error.Message != "" {
		msg = body.Error.Message
	}
	if len(msg) > 300 {
		msg = msg[:300] + "..."
	}

	err := fmt.Sprintf("OpenRouter returned %s", resp.Status)
	if msg != "" {
		err += ": " + msg
	}
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		err += "\nCheck that OPENROUTER_API_KEY holds a valid key."
	case http.StatusPaymentRequired:
		err += "\nThe account is out of credits; a :free model costs nothing."
	case http.StatusTooManyRequests:
		err += "\nFree models are rate limited; wait a moment or try another --model."
	case http.StatusBadRequest, http.StatusNotFound:
		if strings.Contains(strings.ToLower(msg), "model") {
			err += "\nFree models are listed at https://openrouter.ai/models?q=free"
		}
	}
	return errors.New(err)
}

// trackingWriter remembers how much was written and the last byte, so we can
// end the output with a newline and spot empty responses.
type trackingWriter struct {
	w    io.Writer
	n    int
	last byte
}

func (t *trackingWriter) Write(p []byte) (int, error) {
	n, err := t.w.Write(p)
	if n > 0 {
		t.n += n
		t.last = p[n-1]
	}
	return n, err
}
