package main

import (
	"os"
	"testing"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
)

func TestTodoListOriginRestrictsCloudQueueToMicrosoftLists(t *testing.T) {
	a := newTodoTestApp(t)
	if _, err := a.writeTodoSettings(func(todo map[string]any) { todo["syncMode"] = todoSyncMicrosoft }); err != nil {
		t.Fatal(err)
	}
	if err := a.writeTodoTokenStore(todoTokenStore{ClientID: "client", RefreshToken: "refresh"}); err != nil {
		t.Fatal(err)
	}

	local := todoListCache{Version: 1, ListID: todoLocalGroceryListID, PendingOps: []todoPendingOp{}}
	a.enqueueTodoOp(&local, todoPendingOp{Op: "create", ListID: todoLocalGroceryListID, TaskID: "local-1"})
	if len(local.PendingOps) != 0 {
		t.Fatalf("linked Microsoft mode queued a built-in local list: %#v", local.PendingOps)
	}

	remote := todoListInfo{ID: "remote-groceries", DisplayName: "Groceries", Origin: todoListOriginMicrosoft}
	if err := a.todoUpsertListInfo(remote); err != nil {
		t.Fatal(err)
	}
	cloud := todoListCache{Version: 1, ListID: remote.ID, PendingOps: []todoPendingOp{}}
	a.enqueueTodoOp(&cloud, todoPendingOp{Op: "create", ListID: remote.ID, TaskID: "local-2"})
	if len(cloud.PendingOps) != 1 || cloud.PendingOps[0].ListID != remote.ID {
		t.Fatalf("Microsoft-origin list did not queue cloud work: %#v", cloud.PendingOps)
	}
}

func TestTodoMigrationSnapshotExpiresButSourceStaysRetired(t *testing.T) {
	a := newTodoTestApp(t)
	source, ok := a.todoListInfoByID(todoLocalTodoListID)
	if !ok || todoListOriginOf(source) != todoListOriginLocal {
		t.Fatalf("missing default local source: %#v ok=%v", source, ok)
	}
	cache, err := a.upsertTodoTask(source.ID, map[string]any{"title": "Milk"})
	if err != nil {
		t.Fatal(err)
	}
	archive, err := a.todoStageArchive(source, cache, "local-to-microsoft-copy")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.todoCommitArchive(archive); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(a.todoArchiveSnapshotPath(archive.ID)); err != nil {
		t.Fatalf("archive snapshot missing: %v", err)
	}
	if _, ok := a.todoListInfoByID(source.ID); ok {
		t.Fatalf("archived source remained selectable in active list index")
	}

	idx := a.readTodoArchiveIndex()
	idx.Items[0].ExpiresAt = time.Now().Add(-time.Minute).UnixMilli()
	if err := fileio.WriteJSON(a.todoArchiveIndexPath(), idx); err != nil {
		t.Fatal(err)
	}
	a.todoPurgeExpiredArchives()
	if _, err := os.Stat(a.todoArchiveSnapshotPath(archive.ID)); !os.IsNotExist(err) {
		t.Fatalf("expired archive snapshot retained: %v", err)
	}
	if _, ok := a.todoListInfoByID(source.ID); ok {
		t.Fatalf("retired source returned to the active list index after its snapshot expired")
	}
	idx = a.readTodoArchiveIndex()
	if len(idx.Items) != 1 || idx.Items[0].PurgedAt == 0 {
		t.Fatalf("expired archive should retain only a retirement marker: %#v", idx.Items)
	}
}
