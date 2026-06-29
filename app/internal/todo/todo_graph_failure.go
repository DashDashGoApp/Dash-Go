package todo

import (
	"fmt"
	"net/http"
	"strings"
)

// todoGraphFailure carries safe, machine-readable Graph diagnostics across the
// queue and delta layers. It intentionally never retains Graph's free-form
// message text, an access token, a task body, or an opaque continuation URL.
type todoGraphFailure struct {
	Stage  string
	ListID string
	Meta   todoGraphResponseMeta
	Cause  error
	Label  string
}

func (e *todoGraphFailure) Error() string {
	if e == nil {
		return "Microsoft To Do request failed"
	}
	label := strings.TrimSpace(e.Label)
	if label == "" {
		label = "Microsoft To Do request failed"
	}
	parts := make([]string, 0, 2)
	if e.Meta.Status > 0 {
		parts = append(parts, fmt.Sprintf("HTTP %d", e.Meta.Status))
	}
	code := e.Meta.GraphInnerCode
	if code == "" {
		code = e.Meta.GraphCode
	}
	if code != "" {
		parts = append(parts, code)
	}
	if len(parts) == 0 && e.Cause != nil {
		return label + " (network or local transport error)"
	}
	if len(parts) == 0 {
		return label
	}
	return label + " (" + strings.Join(parts, " · ") + ")"
}

func todoGraphFailureFor(stage, listID, label string, payload map[string]any, meta todoGraphResponseMeta, cause error) *todoGraphFailure {
	if meta.GraphCode == "" && meta.GraphInnerCode == "" {
		meta.GraphCode, meta.GraphInnerCode = todoGraphErrorCodes(payload)
	}
	return &todoGraphFailure{Stage: stage, ListID: listID, Meta: meta, Cause: cause, Label: label}
}

func todoGraphFailureFromError(stage, listID, label string, err error) *todoGraphFailure {
	if existing, ok := err.(*todoGraphFailure); ok && existing != nil {
		return existing
	}
	return &todoGraphFailure{Stage: stage, ListID: listID, Cause: err, Label: label}
}

func todoGraphIsThrottle(meta todoGraphResponseMeta) bool {
	return meta.Status == http.StatusTooManyRequests
}

func todoGraphNeedsDeltaReset(meta todoGraphResponseMeta) bool {
	if meta.Status == http.StatusGone {
		return true
	}
	for _, code := range []string{meta.GraphCode, meta.GraphInnerCode} {
		if strings.EqualFold(strings.TrimSpace(code), "syncStateNotFound") {
			return true
		}
	}
	return false
}

func todoApplyGraphFailure(result *todoSyncListResult, stage string, err error) {
	if result == nil {
		return
	}
	failure := todoGraphFailureFromError(stage, result.ListID, "Microsoft To Do sync failed", err)
	result.Stage = failure.Stage
	result.Error = failure.Error()
	result.HTTPStatus = failure.Meta.Status
	result.GraphCode = failure.Meta.GraphCode
	result.GraphInnerCode = failure.Meta.GraphInnerCode
	result.RequestID = failure.Meta.RequestID
	result.RetryAfterSeconds = int(failure.Meta.RetryAfter.Seconds())
	if result.RetryAfterSeconds < 0 {
		result.RetryAfterSeconds = 0
	}
}
