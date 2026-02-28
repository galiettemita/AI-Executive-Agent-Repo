package streaming

import (
	"strings"
	"testing"
)

func TestStreamingConfigLifecycle(t *testing.T) {
	t.Parallel()

	s := NewService()

	cfg := s.UpsertConfig("ws_1", Config{
		AckEnabled:            true,
		TypingIndicator:       true,
		FirstByteSLAMillis:    450,
		ChunkSizeBytes:        4096,
		ProgressiveDisclosure: true,
	})
	if cfg.WorkspaceID != "ws_1" {
		t.Fatalf("unexpected workspace on config: %#v", cfg)
	}

	loaded, ok := s.GetConfig("ws_1")
	if !ok {
		t.Fatalf("expected streaming config lookup success")
	}
	if loaded.FirstByteSLAMillis != 450 {
		t.Fatalf("unexpected streaming config: %#v", loaded)
	}
}

func TestStreamingConfigClampsSLAAndChunkSize(t *testing.T) {
	t.Parallel()

	s := NewService()
	cfg := s.UpsertConfig("ws_clamp", Config{
		FirstByteSLAMillis: 900,
		ChunkSizeBytes:     25000,
	})
	if cfg.FirstByteSLAMillis != 500 {
		t.Fatalf("expected SLA clamp to 500, got %d", cfg.FirstByteSLAMillis)
	}
	if cfg.ChunkSizeBytes != 8192 {
		t.Fatalf("expected chunk size clamp to 8192, got %d", cfg.ChunkSizeBytes)
	}
}

func TestPrepareDeliveryPlanProgressiveDisclosureAndSLA(t *testing.T) {
	t.Parallel()

	s := NewService()
	s.UpsertConfig("ws_plan", Config{
		AckEnabled:            true,
		TypingIndicator:       true,
		FirstByteSLAMillis:    300,
		ChunkSizeBytes:        5,
		ProgressiveDisclosure: true,
	})

	plan := s.PrepareDeliveryPlan("ws_plan", "turn_1", "hello world", 420)
	if !plan.SLABreached {
		t.Fatalf("expected SLA breach in plan: %#v", plan)
	}
	if plan.AckMessage == "" {
		t.Fatalf("expected ack message when ack enabled: %#v", plan)
	}
	if len(plan.Chunks) != 3 {
		t.Fatalf("expected chunked progressive payload, got %d chunks", len(plan.Chunks))
	}
	if !strings.Contains(strings.Join(plan.Events, ","), "BREVIO.streaming.first_byte.v1") {
		t.Fatalf("expected first-byte event in plan events: %#v", plan.Events)
	}

	recent := s.ListRecentPlans("ws_plan")
	if len(recent) != 1 {
		t.Fatalf("expected one stored plan, got %d", len(recent))
	}
	stats := s.GetStats("ws_plan")
	if stats.PlansTotal != 1 || stats.FirstByteSLABreachTotal != 1 || stats.AckSentTotal != 1 {
		t.Fatalf("unexpected streaming stats: %#v", stats)
	}
}

func TestPrepareDeliveryPlanWithoutProgressiveDisclosure(t *testing.T) {
	t.Parallel()

	s := NewService()
	s.UpsertConfig("ws_single", Config{
		AckEnabled:            false,
		TypingIndicator:       false,
		FirstByteSLAMillis:    500,
		ChunkSizeBytes:        4,
		ProgressiveDisclosure: false,
	})

	plan := s.PrepareDeliveryPlan("ws_single", "turn_2", "abcd1234", 120)
	if len(plan.Chunks) != 1 {
		t.Fatalf("expected single chunk without progressive disclosure, got %d", len(plan.Chunks))
	}
	if plan.AckMessage != "" {
		t.Fatalf("expected empty ack message when ack disabled: %#v", plan)
	}
}
