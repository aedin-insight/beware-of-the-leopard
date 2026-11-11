package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testKey = "sk-or-test-not-a-real-key"

func env(key string) func(string) string {
	return func(name string) string {
		if name == "OPENROUTER_API_KEY" {
			return key
		}
		return ""
	}
}

// fakeOpenRouter points the CLI at a test server for the duration of the test.
func fakeOpenRouter(t *testing.T, h http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	old := endpoint
	endpoint = srv.URL
	t.Cleanup(func() { endpoint = old })
}

func sse(w http.ResponseWriter, lines ...string) {
	w.Header().Set("Content-Type", "text/event-stream")
	for _, l := range lines {
		fmt.Fprintf(w, "%s\n\n", l)
		w.(http.Flusher).Flush()
	}
}

func delta(s string) string {
	return fmt.Sprintf(`data: {"choices":[{"delta":{"content":%q}}]}`, s)
}

const stop = `data: {"choices":[{"delta":{"content":""},"finish_reason":"stop"}]}`

type result struct {
	code           int
	stdout, stderr string
}

func ask(ctx context.Context, args []string, stdin string, piped bool, key string) result {
	var out, errOut bytes.Buffer
	code := run(ctx, args, strings.NewReader(stdin), piped, &out, &errOut, env(key))
	return result{code, out.String(), errOut.String()}
}

func TestStreamsReply(t *testing.T) {
	var got struct {
		Model    string                     `json:"model"`
		Stream   bool                       `json:"stream"`
		Messages []struct{ Content string } `json:"messages"`
	}
	var auth string
	fakeOpenRouter(t, func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		json.NewDecoder(r.Body).Decode(&got)
		sse(w, ": OPENROUTER PROCESSING", delta("Hello"), delta(", leopard"), stop, "data: [DONE]")
	})

	res := ask(context.Background(), []string{"say", "hi", "--model", "x/y:free"}, "", false, testKey)

	if res.code != exitOK || res.stdout != "Hello, leopard\n" || res.stderr != "" {
		t.Fatalf("got %+v", res)
	}
	if auth != "Bearer "+testKey || got.Model != "x/y:free" || !got.Stream || got.Messages[0].Content != "say hi" {
		t.Fatalf("bad request: auth=%q body=%+v", auth, got)
	}
}

func TestPromptFromStdin(t *testing.T) {
	var prompt string
	fakeOpenRouter(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Messages []struct{ Content string } }
		json.NewDecoder(r.Body).Decode(&body)
		prompt = body.Messages[0].Content
		sse(w, delta("ok\n"), stop, "data: [DONE]")
	})

	res := ask(context.Background(), nil, "from a pipe\n", true, testKey)

	if res.code != exitOK || res.stdout != "ok\n" || prompt != "from a pipe\n" {
		t.Fatalf("got %+v, prompt %q", res, prompt)
	}
}

func TestFailures(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		stdout  string
		stderr  string
	}{
		{
			name: "bad key",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprint(w, `{"error":{"message":"No auth credentials found","code":401}}`)
			},
			stderr: "401 Unauthorized: No auth credentials found\nCheck that OPENROUTER_API_KEY",
		},
		{
			name: "rate limited",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusTooManyRequests)
				fmt.Fprint(w, `{"error":{"message":"Rate limit exceeded","code":429}}`)
			},
			stderr: "Free models are rate limited",
		},
		{
			name: "non-JSON error page",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "bad gateway", http.StatusBadGateway)
			},
			stderr: "502 Bad Gateway: bad gateway",
		},
		{
			name: "error mid-stream",
			handler: func(w http.ResponseWriter, r *http.Request) {
				sse(w, delta("partial"), `data: {"error":{"message":"provider overloaded"},"choices":[{"delta":{"content":""},"finish_reason":"error"}]}`)
			},
			stdout: "partial\n",
			stderr: "the model failed mid-response: provider overloaded",
		},
		{
			name: "connection dropped",
			handler: func(w http.ResponseWriter, r *http.Request) {
				sse(w, delta("half a sen"))
			},
			stdout: "half a sen\n",
			stderr: "ended before the model finished",
		},
		{
			name: "empty reply",
			handler: func(w http.ResponseWriter, r *http.Request) {
				sse(w, stop, "data: [DONE]")
			},
			stderr: "empty response",
		},
		{
			name: "garbage in stream",
			handler: func(w http.ResponseWriter, r *http.Request) {
				sse(w, "data: {not json")
			},
			stderr: "unreadable data",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeOpenRouter(t, tt.handler)
			res := ask(context.Background(), []string{"hi"}, "", false, testKey)
			if res.code != exitFailure || res.stdout != tt.stdout || !strings.Contains(res.stderr, tt.stderr) {
				t.Fatalf("got %+v", res)
			}
			if strings.Contains(res.stderr, testKey) {
				t.Fatal("API key leaked into stderr")
			}
		})
	}
}

func TestStalledStreamTimesOut(t *testing.T) {
	old := idleTimeout
	idleTimeout = 100 * time.Millisecond
	t.Cleanup(func() { idleTimeout = old })
	fakeOpenRouter(t, func(w http.ResponseWriter, r *http.Request) {
		sse(w, delta("thinking"))
		<-r.Context().Done()
	})

	res := ask(context.Background(), []string{"hi"}, "", false, testKey)

	if res.code != exitFailure || !strings.Contains(res.stderr, "no data from OpenRouter") || res.stdout != "thinking\n" {
		t.Fatalf("got %+v", res)
	}
}

func TestInterrupt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	fakeOpenRouter(t, func(w http.ResponseWriter, r *http.Request) {
		sse(w, delta("going"))
		time.AfterFunc(50*time.Millisecond, cancel) // Ctrl-C once the client has read the chunk
		<-r.Context().Done()
	})

	res := ask(ctx, []string{"hi"}, "", false, testKey)

	if res.code != exitInterrupted || res.stdout != "going\n" || res.stderr != "ask: interrupted\n" {
		t.Fatalf("got %+v", res)
	}
}

func TestUsageErrors(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		stdin  string
		piped  bool
		key    string
		stderr string
	}{
		{"no prompt", nil, "", false, testKey, "Usage: ask"},
		{"empty pipe", nil, "  \n", true, testKey, "the prompt is empty"},
		{"missing key", []string{"hi"}, "", false, "", "OPENROUTER_API_KEY is not set"},
		{"unknown flag", []string{"--temp", "1", "hi"}, "", false, testKey, `unknown flag "--temp"`},
		{"model without value", []string{"hi", "-m"}, "", false, testKey, "-m needs a model name"},
		{"empty model", []string{"--model=", "hi"}, "", false, testKey, "--model needs a model name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeOpenRouter(t, func(w http.ResponseWriter, r *http.Request) {
				t.Error("no request should be sent")
			})
			res := ask(context.Background(), tt.args, tt.stdin, tt.piped, tt.key)
			if res.code != exitUsage || res.stdout != "" || !strings.Contains(res.stderr, tt.stderr) {
				t.Fatalf("got %+v", res)
			}
		})
	}
}

func TestParseArgs(t *testing.T) {
	model, prompt, err := parseArgs([]string{"-m", "a/b:free", "--", "-5", "is", "negative"})
	if err != nil || model != "a/b:free" || prompt != "-5 is negative" {
		t.Fatalf("got %q %q %v", model, prompt, err)
	}
	model, _, _ = parseArgs([]string{"hi"})
	if model != defaultModel || !strings.HasSuffix(defaultModel, ":free") {
		t.Fatalf("default model %q", model)
	}
}
