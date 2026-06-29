package todo

import (
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func testTodoService(t *testing.T) *Service {
	t.Helper()
	var mu sync.Mutex
	settings := map[string]any{}
	clone := func(value map[string]any) map[string]any {
		out := map[string]any{}
		for key, item := range value {
			out[key] = item
		}
		return out
	}
	return New(ServiceConfig{
		TodoDir:   filepath.Join(t.TempDir(), "todo"),
		TokenFile: filepath.Join(t.TempDir(), "todo-token.json"),
		LoadSettings: func() map[string]any {
			mu.Lock()
			defer mu.Unlock()
			return clone(settings)
		},
		UpdateSettings: func(mut func(map[string]any)) (map[string]any, error) {
			mu.Lock()
			defer mu.Unlock()
			mut(settings)
			return clone(settings), nil
		},
		MutateSettings: func(mut func(map[string]any) error) (map[string]any, error) {
			mu.Lock()
			defer mu.Unlock()
			if err := mut(settings); err != nil {
				return nil, err
			}
			return clone(settings), nil
		},
		PeoplePayload: func() map[string]any { return map[string]any{"people": []any{}} },
		Now:           time.Now,
	})
}

func TestServiceKeepsBuiltInGroceryTaskLocalAndRemembersCompletedTitle(t *testing.T) {
	service := testTodoService(t)
	cache, err := service.UpsertTask(LocalGroceryListID, map[string]any{"id": "milk", "title": "Milk"})
	if err != nil {
		t.Fatal(err)
	}
	if len(cache.PendingOps) != 0 || len(cache.Tasks) != 1 || cache.Tasks[0].Pending != "" {
		t.Fatalf("built-in grocery task must stay local: %#v", cache)
	}
	cache.Tasks[0].Status = "completed"
	if err := service.WriteListCache(cache); err != nil {
		t.Fatal(err)
	}
	_, cleared, err := service.ClearCompletedSnapshot(LocalGroceryListID, []string{"milk"})
	if err != nil {
		t.Fatal(err)
	}
	if cleared != 1 {
		t.Fatalf("cleared=%d, want one completed grocery task", cleared)
	}
	memory := service.GroceryMemory()
	if len(memory) != 1 || memory[0].Title != "Milk" {
		t.Fatalf("completed grocery task was not remembered: %#v", memory)
	}
}

func TestServiceOwnsInboundQueueState(t *testing.T) {
	service := testTodoService(t)
	if _, err := service.WriteSettings(func(todo map[string]any) {
		todo["syncMode"] = SyncMicrosoft
		todo["source"] = SyncMicrosoft
		todo["map"] = map[string]any{"todo": "remote", "grocery": LocalGroceryListID}
	}); err != nil {
		t.Fatal(err)
	}
	if err := service.WriteTokenStore(TokenStore{ClientID: "client", RefreshToken: "refresh"}); err != nil {
		t.Fatal(err)
	}
	if err := service.UpsertListInfo(ListInfo{ID: "remote", DisplayName: "Remote", Origin: ListOriginMicrosoft}); err != nil {
		t.Fatal(err)
	}
	service.SetInboundRunningForTest(true)
	prepared, started, err := service.BeginInboundRun([]string{"remote"}, "test")
	if err != nil || started || len(prepared) != 1 || prepared[0] != "remote" {
		t.Fatalf("busy inbound run=%#v started=%v err=%v", prepared, started, err)
	}
	status := service.InboundSyncStatus()
	if !status.Queued || status.CoalescedRequests != 1 {
		t.Fatalf("queued state=%#v", status)
	}
}
