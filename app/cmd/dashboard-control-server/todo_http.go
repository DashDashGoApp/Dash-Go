package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
	todopkg "github.com/DashDashGoApp/Dash-Go/app/internal/todo"
)

const todoSyncResponseWriteWindow = 90 * time.Second

// todoSSEHeartbeatInterval is a package variable only so focused tests can
// exercise the bounded stream lifecycle without sleeping for the production
// cadence. Production keeps the calm 30-second EventSource heartbeat.
var todoSSEHeartbeatInterval = 30 * time.Second

func (a *app) handleTodoGet(w http.ResponseWriter, r *http.Request, path string) bool {
	switch path {
	case "/api/todo/status":
		// Cache/settings only. This must not start a Graph request on dashboard landing.
		a.json(w, a.todoStatusPayload())
		return true
	case "/api/todo/lists":
		a.json(w, a.readTodoListsIndex())
		return true
	case "/api/todo/dock":
		// Dashboard ticker data is cache-only. Do not wake Graph or schedule a
		// provider refresh merely because the dashboard is visible.
		a.json(w, a.todoDashboardDockSummary())
		return true
	case "/api/todo/stream":
		a.handleTodoStream(w, r)
		return true
	}
	return a.handleTodoTasksGet(w, r, path)
}
func (a *app) handleTodoPost(w http.ResponseWriter, r *http.Request, path string, body map[string]any) bool {
	if !strings.HasPrefix(path, "/api/todo/") {
		return false
	}
	if a.handleTodoGroceryMemoryPost(w, r, path, body) {
		return true
	}
	manage := map[string]bool{
		"/api/todo/auth/start": true, "/api/todo/auth/cancel": true, "/api/todo/unlink": true,
		"/api/todo/lists": true, "/api/todo/lists/refresh": true, "/api/todo/map": true, "/api/todo/cadence": true, "/api/todo/inbound-sync": true, "/api/todo/source": true,
		"/api/todo/dock": true, "/api/todo/dock/slots": true, "/api/todo/migrate": true, "/api/todo/sync": true,
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	blockedWriteAction := len(parts) == 5 && parts[0] == "api" && parts[1] == "todo" && parts[2] == "lists" && parts[4] == "sync-failures"
	if (manage[path] || blockedWriteAction) && !a.tokenOK(r.Header.Get("X-Dashboard-Token")) {
		a.err(w, "locked", http.StatusUnauthorized)
		return true
	}
	if blockedWriteAction {
		cache, resolved, err := a.todoResolveBlockedPendingOps(parts[3], jsonutil.BodyString(body, "action"))
		if err != nil {
			a.err(w, err.Error(), http.StatusBadRequest)
			return true
		}
		a.json(w, map[string]any{"ok": true, "resolved": resolved, "cache": cache, "status": a.todoStatusPayload()})
		return true
	}
	switch path {
	case "/api/todo/auth/start":
		clientID := jsonutil.BodyString(body, "clientId")
		if err := todopkg.ValidateClientID(clientID); err != nil {
			a.err(w, err.Error(), http.StatusBadRequest)
			return true
		}
		res, err := a.startTodoAuth(clientID)
		if err != nil {
			a.err(w, err.Error(), http.StatusBadRequest)
			return true
		}
		a.json(w, res)
		return true
	case "/api/todo/auth/cancel":
		a.cancelTodoAuth()
		a.json(w, map[string]any{"ok": true})
		return true
	case "/api/todo/unlink":
		if a.todoHasMappedMicrosoftList() {
			a.err(w, "move each Microsoft-mapped tile to a local list before unlinking", http.StatusConflict)
			return true
		}
		if err := a.unlinkTodo(); err != nil {
			a.err(w, err.Error(), 500)
			return true
		}
		a.json(w, a.todoStatusPayload())
		return true
	case "/api/todo/dock":
		enabled, ok := body["enabled"].(bool)
		if !ok {
			a.err(w, "enabled must be true or false", http.StatusBadRequest)
			return true
		}
		if _, err := a.writeTodoSettings(func(todo map[string]any) { todo["dashboardDock"] = enabled }); err != nil {
			a.err(w, err.Error(), http.StatusInternalServerError)
			return true
		}
		a.json(w, a.todoStatusPayload())
		return true
	case "/api/todo/dock/slots":
		raw, ok := body["slots"].(map[string]any)
		if !ok || len(raw) == 0 {
			a.err(w, "slots must contain one or more dashboard list choices", http.StatusBadRequest)
			return true
		}
		next := a.todoDashboardDockSlots()
		for slot, value := range raw {
			if slot != "todo" && slot != "grocery" {
				a.err(w, "unknown dashboard dock slot", http.StatusBadRequest)
				return true
			}
			enabled, isBool := value.(bool)
			if !isBool {
				a.err(w, "dashboard dock slots must be true or false", http.StatusBadRequest)
				return true
			}
			next[slot] = enabled
		}
		if !next["todo"] && !next["grocery"] {
			a.err(w, "choose at least one list for the Bottom Lists dock", http.StatusBadRequest)
			return true
		}
		if _, err := a.writeTodoSettings(func(todo map[string]any) {
			todo["dashboardDockSlots"] = map[string]any{"todo": next["todo"], "grocery": next["grocery"]}
		}); err != nil {
			a.err(w, err.Error(), http.StatusInternalServerError)
			return true
		}
		a.json(w, a.todoStatusPayload())
		return true
	case "/api/todo/source":
		mode := strings.ToLower(jsonutil.BodyString(body, "syncMode"))
		if mode != todoSyncLocal && mode != todoSyncMicrosoft {
			a.err(w, "syncMode must be local or microsoft", 400)
			return true
		}
		if _, err := a.writeTodoSettings(func(todo map[string]any) {
			todo["source"] = todoSyncLocal
			todo["syncMode"] = mode
		}); err != nil {
			a.err(w, err.Error(), 500)
			return true
		}
		a.todoNotifyInboundScheduler()
		a.json(w, a.todoStatusPayload())
		return true
	case "/api/todo/inbound-sync":
		// Compatibility for a cached pre-beta.60 Control bundle. The old route
		// no longer exposes a tuning decision: every caller converges to the
		// fixed 25-second coordinator cadence.
		if _, err := a.writeTodoSettings(func(todo map[string]any) { todo["inboundSyncSeconds"] = todoInboundSyncFixedSeconds }); err != nil {
			a.err(w, err.Error(), http.StatusInternalServerError)
			return true
		}
		a.todoNotifyInboundScheduler()
		a.json(w, a.todoStatusPayload())
		return true
	case "/api/todo/lists/refresh":
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()
		if err := a.syncTodoListsNow(ctx); err != nil {
			var throttled *todoThrottleError
			if errors.As(err, &throttled) {
				if seconds := int(throttled.RetryAfter.Seconds()); seconds > 0 {
					w.Header().Set("Retry-After", strconv.Itoa(seconds))
				}
				a.err(w, "Microsoft asked Dash-Go to wait before refreshing available lists", http.StatusTooManyRequests)
				return true
			}
			a.err(w, err.Error(), http.StatusBadGateway)
			return true
		}
		a.json(w, map[string]any{"ok": true, "lists": a.readTodoListsIndex().Lists})
		return true
	case "/api/todo/lists":
		name := jsonutil.BodyString(body, "displayName")
		if name == "" {
			a.err(w, "list name required", 400)
			return true
		}
		if err := todopkg.ValidateListDisplayName(name); err != nil {
			a.err(w, err.Error(), http.StatusBadRequest)
			return true
		}
		item := todoListInfo{}
		if a.todoCloudSyncEnabled() {
			ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
			defer cancel()
			created, err := a.todoCreateCloudList(ctx, name)
			if err != nil {
				a.err(w, err.Error(), 502)
				return true
			}
			item = created
		} else {
			id := jsonutil.BodyString(body, "id")
			if id == "" {
				id = fmt.Sprintf("local-list-%d", time.Now().UnixNano())
			}
			if err := todopkg.ValidateListID(id); err != nil {
				a.err(w, err.Error(), http.StatusBadRequest)
				return true
			}
			item = todoListInfo{ID: id, DisplayName: name, Origin: todoListOriginLocal}
		}
		idx := a.readTodoListsIndex()
		idx.Lists = append(idx.Lists, item)
		if err := a.writeTodoListsIndex(idx); err != nil {
			a.err(w, err.Error(), 500)
			return true
		}
		a.todoEmit(map[string]any{"type": "list.upsert", "listId": item.ID})
		a.json(w, map[string]any{"ok": true, "list": item})
		return true
	case "/api/todo/migrate":
		ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
		defer cancel()
		result, err := a.migrateTodoSlot(ctx, jsonutil.BodyString(body, "action"), jsonutil.BodyString(body, "slot"))
		if err != nil {
			a.err(w, err.Error(), http.StatusBadRequest)
			return true
		}
		a.json(w, result)
		return true
	case "/api/todo/map":
		for _, slot := range []string{"todo", "grocery"} {
			if value, exists := body[slot]; exists {
				listID := jsonutil.StringValue(value)
				if listID != "" {
					if _, ok := a.todoListInfoByID(listID); !ok {
						a.err(w, "mapped list is not available", http.StatusBadRequest)
						return true
					}
				}
			}
		}
		_, err := a.writeTodoSettings(func(todo map[string]any) {
			m, _ := todo["map"].(map[string]any)
			if m == nil {
				m = map[string]any{}
			}
			for _, slot := range []string{"todo", "grocery"} {
				if value, exists := body[slot]; exists {
					m[slot] = jsonutil.StringValue(value)
				}
			}
			todo["map"] = m
		})
		if err != nil {
			a.err(w, err.Error(), 500)
			return true
		}
		a.todoNotifyInboundScheduler()
		a.json(w, a.todoStatusPayload())
		return true
	case "/api/todo/lists/clear-completed":
		listID := jsonutil.BodyString(body, "listId")
		if listID == "" {
			a.err(w, "listId required", 400)
			return true
		}
		if err := todopkg.ValidateListID(listID); err != nil {
			a.err(w, err.Error(), http.StatusBadRequest)
			return true
		}
		if err := todopkg.ValidateTaskIDs(body["taskIds"]); err != nil {
			a.err(w, err.Error(), http.StatusBadRequest)
			return true
		}
		cache, cleared, err := a.clearTodoCompletedSnapshot(listID, todoTaskIDsFromBody(body["taskIds"]))
		if err != nil {
			a.err(w, err.Error(), 500)
			return true
		}
		a.todoEmit(map[string]any{"type": "task.clearCompleted", "listId": listID})
		a.json(w, map[string]any{"ok": true, "cleared": cleared, "cache": cache, "groceryMemory": a.todoGroceryMemory()})
		return true
	case "/api/todo/cadence":
		if _, err := a.writeTodoSettings(func(todo map[string]any) { todo["cadence"] = body }); err != nil {
			a.err(w, err.Error(), 500)
			return true
		}
		a.json(w, a.todoStatusPayload())
		return true
	case "/api/todo/sync":
		ctx, cancel := context.WithTimeout(r.Context(), 75*time.Second)
		defer cancel()
		// This handler may legitimately run past the server-wide 45s
		// WriteTimeout; give the response enough room to be written after a
		// full-length sync instead of severing the connection mid-flight.
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(todoSyncResponseWriteWindow))
		result, err := a.todoRunInboundSync(ctx)
		if err != nil {
			status := http.StatusBadGateway
			var wait *todoInboundBackoffError
			if errors.As(err, &wait) {
				status = http.StatusTooManyRequests
			}
			var throttled *todoThrottleError
			if errors.As(err, &throttled) {
				status = http.StatusTooManyRequests
				if throttled.RetryAfter > 0 {
					w.Header().Set("Retry-After", strconv.Itoa(int(throttled.RetryAfter.Seconds())))
				}
			}
			a.err(w, err.Error(), status)
			return true
		}
		a.json(w, map[string]any{"ok": result.OK, "result": result, "inboundSync": a.todoInboundSyncStatus(), "summary": todoSyncResultText(result)})
		return true
	}
	if a.handleTodoTasksPost(w, r, parts, body) {
		return true
	}
	a.err(w, "unknown todo endpoint", http.StatusNotFound)
	return true
}
func (a *app) handleTodoStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		a.err(w, "stream unsupported", 500)
		return
	}
	// The server-wide WriteTimeout (45s) would otherwise sever this long-lived
	// stream and force EventSource reconnect churn. Clear the write deadline
	// for this response only; other endpoints keep the global limit.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	ch := make(chan []byte, 8)
	a.todoStreamMu.Lock()
	if a.todoStreams == nil {
		a.todoStreams = map[chan []byte]bool{}
	}
	a.todoStreams[ch] = true
	a.todoStreamMu.Unlock()
	defer func() { a.todoStreamMu.Lock(); delete(a.todoStreams, ch); a.todoStreamMu.Unlock(); close(ch) }()
	fmt.Fprintf(w, "event: sync.state\ndata: {}\n\n")
	flusher.Flush()
	// Heartbeat comments let the server notice dead peers instead of holding
	// the goroutine until TCP keepalive gives up.
	heartbeat := time.NewTicker(todoSSEHeartbeatInterval)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case msg := <-ch:
			eventName := "todo"
			var payload map[string]any
			if json.Unmarshal(msg, &payload) == nil && jsonutil.StringValue(payload["type"]) == "sync.state" {
				eventName = "sync.state"
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventName, msg)
			flusher.Flush()
		}
	}
}
func (a *app) todoEmit(v map[string]any) {
	b, _ := json.Marshal(v)
	a.todoStreamMu.Lock()
	defer a.todoStreamMu.Unlock()
	for ch := range a.todoStreams {
		select {
		case ch <- b:
		default:
		}
	}
}
