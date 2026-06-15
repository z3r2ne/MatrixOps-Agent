package session

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLLMTracingHTTPClientCapturesRequestAndResponseBodies(t *testing.T) {
	t.Parallel()

	const requestBody = `{"input":"hello"}`
	const responseBody = "data: ok\n\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if string(gotBody) != requestBody {
			t.Fatalf("server got request body %q, want %q", string(gotBody), requestBody)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(responseBody))
	}))
	defer server.Close()

	var startSnapshot llmAPICallSnapshot
	var endSnapshot llmAPICallSnapshot
	var gotRequestBody string
	var gotResponseBody string
	var gotResponseErr error

	client := newLLMTracingHTTPClient(nil, llmTracingHooks{
		OnRequestStart: func(trace *llmAPICallTrace, body string) {
			gotRequestBody = body
			startSnapshot = trace.snapshot()
		},
		OnResponseDone: func(trace *llmAPICallTrace, body string, err error) {
			gotResponseBody = body
			gotResponseErr = err
			endSnapshot = trace.snapshot()
		},
	}, 0, 0)

	resp, err := client.Post(server.URL, "application/json", strings.NewReader(requestBody))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()

	gotResponseBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if string(gotResponseBytes) != responseBody {
		t.Fatalf("client got response body %q, want %q", string(gotResponseBytes), responseBody)
	}

	if gotRequestBody != requestBody {
		t.Fatalf("hook got request body %q, want %q", gotRequestBody, requestBody)
	}
	if gotResponseBody != responseBody {
		t.Fatalf("hook got response body %q, want %q", gotResponseBody, responseBody)
	}
	if gotResponseErr != nil {
		t.Fatalf("hook got response err: %v", gotResponseErr)
	}
	if startSnapshot.RequestID == "" {
		t.Fatal("expected request id on start snapshot")
	}
	if endSnapshot.RequestID == "" || endSnapshot.RequestID != startSnapshot.RequestID {
		t.Fatalf("expected matching request ids, start=%q end=%q", startSnapshot.RequestID, endSnapshot.RequestID)
	}
	if endSnapshot.StatusCode != http.StatusCreated {
		t.Fatalf("response snapshot status code = %d, want %d", endSnapshot.StatusCode, http.StatusCreated)
	}
}
