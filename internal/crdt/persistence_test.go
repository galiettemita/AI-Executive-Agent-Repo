package crdt

import (
	"context"
	"testing"
)

func TestInMemoryPersistenceSaveLoad(t *testing.T) {
	t.Parallel()
	store := NewInMemoryPersistence()
	ctx := context.Background()

	clock := VectorClock{"actor1": 3, "actor2": 1}
	if err := store.Save(ctx, "key1", clock); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := store.Load(ctx, "key1")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if (*loaded)["actor1"] != 3 || (*loaded)["actor2"] != 1 {
		t.Fatalf("unexpected clock: %v", *loaded)
	}
}

func TestInMemoryPersistenceLoadNotFound(t *testing.T) {
	t.Parallel()
	store := NewInMemoryPersistence()
	ctx := context.Background()

	_, err := store.Load(ctx, "missing")
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestInMemoryPersistenceDelete(t *testing.T) {
	t.Parallel()
	store := NewInMemoryPersistence()
	ctx := context.Background()

	_ = store.Save(ctx, "key1", VectorClock{"a": 1})
	if err := store.Delete(ctx, "key1"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	_, err := store.Load(ctx, "key1")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestInMemoryPersistenceListKeys(t *testing.T) {
	t.Parallel()
	store := NewInMemoryPersistence()
	ctx := context.Background()

	_ = store.Save(ctx, "ws1/entity_a", VectorClock{"a": 1})
	_ = store.Save(ctx, "ws1/entity_b", VectorClock{"b": 2})
	_ = store.Save(ctx, "ws2/entity_c", VectorClock{"c": 3})

	keys, err := store.ListKeys(ctx, "ws1/")
	if err != nil {
		t.Fatalf("list keys failed: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d: %v", len(keys), keys)
	}
	if keys[0] != "ws1/entity_a" || keys[1] != "ws1/entity_b" {
		t.Fatalf("unexpected keys: %v", keys)
	}
}

func TestInMemoryPersistenceEmptyKey(t *testing.T) {
	t.Parallel()
	store := NewInMemoryPersistence()
	ctx := context.Background()

	if err := store.Save(ctx, "", VectorClock{"a": 1}); err == nil {
		t.Fatal("expected error for empty key")
	}
	if _, err := store.Load(ctx, ""); err == nil {
		t.Fatal("expected error for empty key")
	}
	if err := store.Delete(ctx, ""); err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestInMemoryPersistenceOverwrite(t *testing.T) {
	t.Parallel()
	store := NewInMemoryPersistence()
	ctx := context.Background()

	_ = store.Save(ctx, "key1", VectorClock{"a": 1})
	_ = store.Save(ctx, "key1", VectorClock{"a": 5, "b": 2})

	loaded, err := store.Load(ctx, "key1")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if (*loaded)["a"] != 5 || (*loaded)["b"] != 2 {
		t.Fatalf("expected overwritten clock, got: %v", *loaded)
	}
}

func TestMergeWithPersistenceNewKey(t *testing.T) {
	t.Parallel()
	store := NewInMemoryPersistence()
	ctx := context.Background()
	incoming := VectorClock{"actor1": 1}

	result, err := MergeWithPersistence(ctx, store, "new_key", incoming)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ConflictDetected {
		t.Fatal("expected no conflict for new key")
	}
	if result.Resolution != "new_key" {
		t.Fatalf("expected resolution new_key, got: %s", result.Resolution)
	}
	if result.Merged["actor1"] != 1 {
		t.Fatalf("unexpected merged clock: %v", result.Merged)
	}
}

func TestMergeWithPersistenceFastForward(t *testing.T) {
	t.Parallel()
	store := NewInMemoryPersistence()
	ctx := context.Background()

	_ = store.Save(ctx, "key1", VectorClock{"actor1": 1})
	incoming := VectorClock{"actor1": 3}

	result, err := MergeWithPersistence(ctx, store, "key1", incoming)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ConflictDetected {
		t.Fatal("expected no conflict for fast forward")
	}
	if result.Resolution != "fast_forward" {
		t.Fatalf("expected resolution fast_forward, got: %s", result.Resolution)
	}
	if result.Merged["actor1"] != 3 {
		t.Fatalf("unexpected merged clock: %v", result.Merged)
	}
}

func TestMergeWithPersistenceConcurrent(t *testing.T) {
	t.Parallel()
	store := NewInMemoryPersistence()
	ctx := context.Background()

	_ = store.Save(ctx, "key1", VectorClock{"actor1": 2, "actor2": 1})
	incoming := VectorClock{"actor1": 1, "actor2": 3}

	result, err := MergeWithPersistence(ctx, store, "key1", incoming)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.ConflictDetected {
		t.Fatal("expected conflict for concurrent writes")
	}
	if result.Resolution != "concurrent_merge" {
		t.Fatalf("expected resolution concurrent_merge, got: %s", result.Resolution)
	}
	if result.Merged["actor1"] != 2 || result.Merged["actor2"] != 3 {
		t.Fatalf("unexpected merged clock: %v", result.Merged)
	}
}

func TestMergeWithPersistenceLocalDominates(t *testing.T) {
	t.Parallel()
	store := NewInMemoryPersistence()
	ctx := context.Background()

	_ = store.Save(ctx, "key1", VectorClock{"actor1": 5})
	incoming := VectorClock{"actor1": 2}

	result, err := MergeWithPersistence(ctx, store, "key1", incoming)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ConflictDetected {
		t.Fatal("expected no conflict when local dominates")
	}
	if result.Resolution != "local_dominates" {
		t.Fatalf("expected resolution local_dominates, got: %s", result.Resolution)
	}
}

func TestMergeWithPersistenceEqual(t *testing.T) {
	t.Parallel()
	store := NewInMemoryPersistence()
	ctx := context.Background()

	_ = store.Save(ctx, "key1", VectorClock{"actor1": 3})
	incoming := VectorClock{"actor1": 3}

	result, err := MergeWithPersistence(ctx, store, "key1", incoming)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ConflictDetected {
		t.Fatal("expected no conflict for equal clocks")
	}
	if result.Resolution != "no_change" {
		t.Fatalf("expected resolution no_change, got: %s", result.Resolution)
	}
}

func TestMergeWithPersistenceNilStore(t *testing.T) {
	t.Parallel()
	_, err := MergeWithPersistence(context.Background(), nil, "key", VectorClock{"a": 1})
	if err == nil {
		t.Fatal("expected error for nil store")
	}
}

func TestMergeWithPersistenceEmptyKey(t *testing.T) {
	t.Parallel()
	store := NewInMemoryPersistence()
	_, err := MergeWithPersistence(context.Background(), store, "", VectorClock{"a": 1})
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestInMemoryPersistenceIsolation(t *testing.T) {
	t.Parallel()
	store := NewInMemoryPersistence()
	ctx := context.Background()

	original := VectorClock{"a": 1}
	_ = store.Save(ctx, "key1", original)

	// Mutate the original — should not affect the stored copy.
	original["a"] = 99

	loaded, _ := store.Load(ctx, "key1")
	if (*loaded)["a"] != 1 {
		t.Fatal("store should be isolated from caller mutations")
	}
}
