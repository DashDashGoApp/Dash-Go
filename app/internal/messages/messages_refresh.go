package messages

import (
	"context"
	"fmt"
	"math/rand"
	"path/filepath"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func (s *Service) messageCachePayload() map[string]any {
	cache := jsonutil.Map(s.readJSONDefault(filepath.Join(s.configDir, "message-cache.json"), map[string]any{"items": []any{}, "generatedAt": 0, "sources": []any{}, "sourceStatus": []any{}}))
	cache["items"] = applyMessageOverrides(jsonutil.List(cache["items"]), s.messageOverrides())
	if _, ok := cache["sourceStatus"].([]any); !ok {
		cache["sourceStatus"] = []any{}
	}
	return cache
}

func (s *Service) messageSourcesStatus() map[string]any {
	return map[string]any{"defs": s.messageDefs(), "prefs": s.messagePrefs(), "cache": s.messageCachePayload(), "overrides": s.messageOverrides(), "generator": "go"}
}

func (s *Service) refreshMessages(ctx context.Context, includeNetwork, manual bool) map[string]any {
	prior := s.messageCachePayload()
	requestedNetwork := includeNetwork
	if includeNetwork && !s.networkLikelyAvailable() {
		includeNetwork = false
	}
	prefs := s.messagePrefs()
	enabledList := s.normalizeMessageEnabled(jsonutil.List(prefs["enabled"]))
	cats := map[string]messageCategory{}
	for _, c := range messageCategories {
		cats[c.ID] = c
	}
	rawItems := []any{}
	used := []any{}
	statuses := []any{}
	for _, raw := range enabledList {
		id := fmt.Sprint(raw)
		c, ok := cats[id]
		if !ok {
			continue
		}
		got := s.fetchMessageCategory(ctx, c, includeNetwork, 8, manual)
		statuses = append(statuses, got.Status)
		if len(got.Items) > 0 {
			used = append(used, id)
			for _, it := range got.Items {
				rawItems = append(rawItems, it)
			}
		}
	}
	items := applyMessageOverrides(rawItems, s.messageOverrides())
	if len(items) > 120 {
		items = items[:120]
	}
	remoteOK := false
	for _, raw := range statuses {
		m := jsonutil.Map(raw)
		if m["ok"] == true && fmt.Sprint(m["servedBy"]) != "local" {
			remoteOK = true
			break
		}
	}
	// During a proven outage or a provider-wide failure, keep the prior
	// successful cache rather than replacing it with a local/error payload.
	if requestedNetwork && !remoteOK && jsonutil.Int(prior["generatedAt"], 0) > 0 {
		prior["refreshDeferred"] = true
		prior["sourceStatus"] = statuses
		return prior
	}
	payload := map[string]any{"items": items, "generatedAt": nowMillis(), "lastSuccessAt": nowMillis(), "sources": used, "enabled": enabledList, "sourceStatus": statuses, "generator": "go"}
	s.saveMessageCache(payload)
	return payload
}

func (s *Service) fetchMessageCategory(ctx context.Context, c messageCategory, includeNetwork bool, want int, manual bool) messageFetchResult {
	errorsOut := []any{}
	skipped := []any{}
	var items []map[string]any
	served := ""
	if includeNetwork {
		for idx, p := range c.Providers {
			if until, reason, _, backed := s.providerBackoffActive("message-" + p); backed {
				skipped = append(skipped, map[string]any{"provider": p, "reason": "retry backoff until " + until.Format(time.RFC3339) + ": " + reason})
				continue
			}
			if !s.messageProviderReady(p) {
				skipped = append(skipped, map[string]any{"provider": p, "reason": "missing optional API key"})
				continue
			}
			attempts := 1
			if idx == 0 {
				attempts = 2
			}
			for range attempts {
				vals, err := s.fetchMessageProvider(ctx, p, want)
				if err != nil {
					s.noteProviderBackoff("message-"+p, err)
					errorsOut = append(errorsOut, map[string]any{"provider": p, "error": truncate(err.Error(), 140)})
					continue
				}
				norm := []map[string]any{}
				for _, t := range vals {
					if it := messageItem(t, c.ID, c.NSFW, 1); it != nil {
						norm = append(norm, it)
					}
				}
				norm = dedupeMessageItems(norm)
				if len(norm) > 0 {
					s.clearProviderBackoff("message-" + p)
					served = p
					items = norm
					break
				}
				errorsOut = append(errorsOut, map[string]any{"provider": p, "error": "empty response"})
			}
			if len(items) > 0 {
				break
			}
		}
	}
	supplemented := false
	if len(items) < want {
		need := want - len(items)
		locals := s.localMessageItems(c.Local, c.ID, c.NSFW, need, manual)
		if served != "" && len(locals) > 0 {
			supplemented = true
		}
		items = append(items, locals...)
		items = dedupeMessageItems(items)
	}
	if len(items) > want {
		items = items[:want]
	}
	if served == "" {
		served = "local"
	}
	ok := len(items) > 0 && (served != "local" || !includeNetwork || len(c.Providers) == 0)
	return messageFetchResult{Items: items, Status: map[string]any{"id": c.ID, "label": c.Label, "ok": ok, "servedBy": served, "servedByLabel": providerLabels[served], "providerCount": len(c.Providers), "skipped": skipped, "errors": errorsOut, "localSupplement": supplemented, "generator": "go"}}
}

func (s *Service) localMessageItems(localKey, source string, nsfw bool, count int, manual bool) []map[string]any {
	pool := append([]string{}, localMessages[localKey]...)
	if len(pool) == 0 || count <= 0 {
		return nil
	}
	seed := time.Now().Unix() / 86400
	if manual {
		seed = time.Now().UnixNano()
	}
	rng := rand.New(rand.NewSource(seed + int64(len(source))))
	rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	count = min(count, len(pool))
	out := []map[string]any{}
	for _, t := range pool[:count] {
		if it := messageItem(t, source, nsfw, 1); it != nil {
			out = append(out, it)
		}
	}
	return out
}

func dedupeMessageItems(items []map[string]any) []map[string]any {
	seen := map[string]bool{}
	out := []map[string]any{}
	for _, it := range items {
		key := normMessage(fmt.Sprint(it["text"]))
		if key != "" && !seen[key] {
			seen[key] = true
			out = append(out, it)
		}
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
