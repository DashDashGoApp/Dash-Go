package main

import (
	"testing"
	"time"
)

func TestTodoInboundCoordinatorCoalescesBusyRequests(t *testing.T) {
	a := newTodoTestApp(t)
	enableTodoMicrosoftTestList(t, a, "remote")
	a.todoSetInboundRunningForTest(true)
	prepared, started, err := a.todoBeginInboundRun([]string{"remote"}, "scheduled")
	if err != nil || started {
		t.Fatalf("busy request started=%v err=%v", started, err)
	}
	if len(prepared) != 1 || prepared[0] != "remote" {
		t.Fatalf("prepared lists=%#v", prepared)
	}
	status := a.todoInboundSyncStatus()
	if !status.Queued || status.CoalescedRequests != 1 {
		t.Fatalf("busy request was not coalesced: %#v", status)
	}
}

func TestTodoInboundCoordinatorStartsOneFollowUp(t *testing.T) {
	a := newTodoTestApp(t)
	enableTodoMicrosoftTestList(t, a, "remote")
	a.todoSetInboundRunningForTest(true)
	a.todoQueueInboundForTest([]string{"remote"}, "scheduled", time.Now().Add(-time.Second))
	next := a.todoFinishInboundRunForTest(nil, time.Now().Add(-10*time.Millisecond))
	if len(next) != 1 || next[0] != "remote" {
		t.Fatalf("follow-up lists=%#v", next)
	}
	status := a.todoInboundSyncStatus()
	if !status.Running || status.Queued || status.LastQueueWaitMs < 1 {
		t.Fatalf("follow-up state=%#v", status)
	}
}
