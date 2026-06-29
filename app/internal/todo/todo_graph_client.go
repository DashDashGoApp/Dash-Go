package todo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func todoGraphPathID(id string) string { return url.PathEscape(strings.TrimSpace(id)) }

type todoGraphResponseMeta struct {
	Status          int
	RetryAfter      time.Duration
	RequestID       string
	ClientRequestID string
	Location        string
	GraphCode       string
	GraphInnerCode  string
}

func (a *Service) todoGraphRequest(ctx context.Context, method, endpoint string, body any, accessToken string) (*http.Request, error) {
	urlText := endpoint
	if !strings.HasPrefix(endpoint, "http") {
		urlText = "https://graph.microsoft.com/v1.0" + endpoint
	}
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, urlText, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	// Microsoft documents Content-Type for both task-list and task delta GETs.
	// Sending it consistently also keeps DELETE/GET/POST/PATCH transport shape
	// deterministic for Graph and for request-fixture regression tests.
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "odata.maxpagesize=50")
	return req, nil
}

func decodeTodoGraphResponse(res *http.Response) map[string]any {
	out := map[string]any{}
	if res != nil && res.Body != nil {
		_ = json.NewDecoder(res.Body).Decode(&out)
	}
	return out
}

func todoRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if at, err := http.ParseTime(value); err == nil && at.After(now) {
		return at.Sub(now)
	}
	return 0
}

func todoGraphErrorCodes(payload map[string]any) (string, string) {
	raw, _ := payload["error"].(map[string]any)
	if raw == nil {
		return "", ""
	}
	code := jsonutil.StringValue(raw["code"])
	innerCode := ""
	for {
		next, ok := raw["innerError"].(map[string]any)
		if !ok {
			next, ok = raw["innererror"].(map[string]any)
		}
		if !ok || next == nil {
			break
		}
		if candidate := jsonutil.StringValue(next["code"]); candidate != "" {
			innerCode = candidate
		}
		raw = next
	}
	return code, innerCode
}

func todoGraphResponseMetaFrom(res *http.Response, payload map[string]any) todoGraphResponseMeta {
	meta := todoGraphResponseMeta{}
	if res != nil {
		meta.Status = res.StatusCode
		meta.RetryAfter = todoRetryAfter(res.Header.Get("Retry-After"), time.Now())
		meta.RequestID = strings.TrimSpace(res.Header.Get("request-id"))
		meta.ClientRequestID = strings.TrimSpace(res.Header.Get("client-request-id"))
		meta.Location = strings.TrimSpace(res.Header.Get("Location"))
	}
	meta.GraphCode, meta.GraphInnerCode = todoGraphErrorCodes(payload)
	return meta
}

// todoGraphResponse preserves the small amount of response metadata needed for
// bounded Graph recovery. It deliberately excludes response bodies from status
// diagnostics so task text, tokens, and opaque delta URLs never reach the UI.
func (a *Service) todoGraphResponse(ctx context.Context, method, endpoint string, body any) (map[string]any, todoGraphResponseMeta, error) {
	store, err := a.refreshTodoToken(ctx)
	if err != nil {
		return nil, todoGraphResponseMeta{}, err
	}
	do := func(token string) (map[string]any, todoGraphResponseMeta, error) {
		request, err := a.todoGraphRequest(ctx, method, endpoint, body, token)
		if err != nil {
			return nil, todoGraphResponseMeta{}, err
		}
		client := &http.Client{Timeout: 25 * time.Second}
		res, err := client.Do(request)
		if err != nil {
			return nil, todoGraphResponseMeta{}, err
		}
		payload := decodeTodoGraphResponse(res)
		meta := todoGraphResponseMetaFrom(res, payload)
		res.Body.Close()
		return payload, meta, nil
	}
	payload, meta, err := do(store.AccessToken)
	if err != nil || meta.Status != http.StatusUnauthorized {
		return payload, meta, err
	}
	store.AccessExpiresAt = 0
	_ = a.writeTodoTokenStore(store)
	store, err = a.refreshTodoToken(ctx)
	if err != nil {
		return payload, meta, err
	}
	return do(store.AccessToken)
}

// todoGraphMeta remains the compact compatibility wrapper for ordinary writes.
func (a *Service) todoGraphMeta(ctx context.Context, method, endpoint string, body any) (map[string]any, int, time.Duration, error) {
	payload, meta, err := a.todoGraphResponse(ctx, method, endpoint, body)
	return payload, meta.Status, meta.RetryAfter, err
}

func (a *Service) todoGraph(ctx context.Context, method, endpoint string, body any) (map[string]any, int, error) {
	payload, status, _, err := a.todoGraphMeta(ctx, method, endpoint, body)
	return payload, status, err
}

// Graph error messages are developer-facing and mutable. Keep UI/health strings
// stable and code-based; detailed response metadata is retained only in the
// narrow, authenticated Sync-now result.
func todoGraphError(payload map[string]any, fallback string) error {
	code, inner := todoGraphErrorCodes(payload)
	if inner != "" {
		return fmt.Errorf("%s (%s)", fallback, inner)
	}
	if code != "" {
		return fmt.Errorf("%s (%s)", fallback, code)
	}
	return fmt.Errorf("%s", fallback)
}
