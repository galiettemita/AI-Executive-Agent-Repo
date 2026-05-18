package edge

import "testing"

func TestRegisterAgent(t *testing.T) {
	t.Parallel()
	s := NewEdgeService()

	agent, err := s.RegisterAgent("ws1", "linux", "1.0.0", []string{"read_cache"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agent.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if agent.Status != "online" {
		t.Fatalf("expected online status, got %s", agent.Status)
	}
}

func TestRegisterAgentMissingWorkspace(t *testing.T) {
	t.Parallel()
	s := NewEdgeService()

	_, err := s.RegisterAgent("", "linux", "1.0.0", nil)
	if err == nil {
		t.Fatal("expected error for empty workspace")
	}
}

func TestUpdateHeartbeat(t *testing.T) {
	t.Parallel()
	s := NewEdgeService()

	agent, _ := s.RegisterAgent("ws1", "linux", "1.0.0", nil)
	if err := s.UpdateHeartbeat(agent.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateHeartbeatNotFound(t *testing.T) {
	t.Parallel()
	s := NewEdgeService()

	err := s.UpdateHeartbeat("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent agent")
	}
}

func TestQueueAndSyncTasks(t *testing.T) {
	t.Parallel()
	s := NewEdgeService()

	agent, _ := s.RegisterAgent("ws1", "linux", "1.0.0", nil)
	task, err := s.QueueOfflineTask(agent.ID, "backup", []byte("data"), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.Status != "queued" {
		t.Fatalf("expected queued status, got %s", task.Status)
	}

	synced, err := s.SyncTasks(agent.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(synced) != 1 {
		t.Fatalf("expected 1 synced task, got %d", len(synced))
	}
	if synced[0].Status != "synced" {
		t.Fatalf("expected synced status, got %s", synced[0].Status)
	}
}

func TestReportTaskResult(t *testing.T) {
	t.Parallel()
	s := NewEdgeService()

	agent, _ := s.RegisterAgent("ws1", "linux", "1.0.0", nil)
	task, _ := s.QueueOfflineTask(agent.ID, "backup", []byte("data"), 1)
	_, _ = s.SyncTasks(agent.ID)

	if err := s.ReportTaskResult(task.ID, true, []byte("ok")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReportTaskResultFailure(t *testing.T) {
	t.Parallel()
	s := NewEdgeService()

	agent, _ := s.RegisterAgent("ws1", "linux", "1.0.0", nil)
	task, _ := s.QueueOfflineTask(agent.ID, "backup", []byte("data"), 1)
	_, _ = s.SyncTasks(agent.ID)

	if err := s.ReportTaskResult(task.ID, false, []byte("error")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetOfflineTier(t *testing.T) {
	t.Parallel()
	s := NewEdgeService()

	a1, _ := s.RegisterAgent("ws1", "linux", "1.0.0", []string{"full_offline"})
	if tier := s.GetOfflineTier(a1.ID); tier != T0FullOffline {
		t.Fatalf("expected T0, got %s", tier)
	}

	a2, _ := s.RegisterAgent("ws1", "linux", "1.0.0", []string{"queue_offline"})
	if tier := s.GetOfflineTier(a2.ID); tier != T1QueueOnly {
		t.Fatalf("expected T1, got %s", tier)
	}

	a3, _ := s.RegisterAgent("ws1", "linux", "1.0.0", []string{"read_cache"})
	if tier := s.GetOfflineTier(a3.ID); tier != T2ReadCache {
		t.Fatalf("expected T2, got %s", tier)
	}

	a4, _ := s.RegisterAgent("ws1", "linux", "1.0.0", nil)
	if tier := s.GetOfflineTier(a4.ID); tier != T3Connected {
		t.Fatalf("expected T3, got %s", tier)
	}

	if tier := s.GetOfflineTier("nonexistent"); tier != T3Connected {
		t.Fatalf("expected T3 for unknown agent, got %s", tier)
	}
}
