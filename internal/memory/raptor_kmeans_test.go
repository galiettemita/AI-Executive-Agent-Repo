package memory

import (
	"math"
	"testing"

	"github.com/google/uuid"
)

func TestKMeans_KFormula_10Episodes(t *testing.T) {
	k := ComputeK(10)
	if k != 2 {
		t.Fatalf("k for 10 episodes: got %d, want 2", k)
	}
}

func TestKMeans_KFormula_40Episodes(t *testing.T) {
	k := ComputeK(40)
	if k != 2 {
		t.Fatalf("k for 40 episodes: got %d, want 2", k)
	}
}

func TestKMeans_KFormula_100Episodes(t *testing.T) {
	k := ComputeK(100)
	expected := int(math.Round(math.Sqrt(100.0 / 10.0))) // sqrt(10) ≈ 3.16 → 3
	if k != expected {
		t.Fatalf("k for 100 episodes: got %d, want %d", k, expected)
	}
}

func TestKMeans_KFormula_250Episodes(t *testing.T) {
	k := ComputeK(250)
	if k != 5 {
		t.Fatalf("k for 250 episodes: got %d, want 5", k)
	}
}

func TestKMeans_KNeverExceedsN(t *testing.T) {
	k := ComputeK(1)
	if k > 1 {
		t.Fatalf("k=%d exceeds n=1", k)
	}
}

func TestKMeans_TwoClearClusters(t *testing.T) {
	eps := make([]Item, 10)
	embs := make([][]float32, 10)
	for i := 0; i < 5; i++ {
		eps[i] = Item{ID: uuid.Must(uuid.NewV7()), Body: "cluster A"}
		embs[i] = nearVec([]float32{1, 0}, float32(i)*0.01)
	}
	for i := 5; i < 10; i++ {
		eps[i] = Item{ID: uuid.Must(uuid.NewV7()), Body: "cluster B"}
		embs[i] = nearVec([]float32{0, 1}, float32(i-5)*0.01)
	}

	clusters := kMeansCluster(eps, embs, 50)
	if len(clusters) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(clusters))
	}
	for i, c := range clusters {
		if len(c) != 5 {
			t.Errorf("cluster %d: len=%d, want 5", i, len(c))
		}
	}
}

func TestKMeans_Deterministic(t *testing.T) {
	eps := make([]Item, 20)
	embs := make([][]float32, 20)
	for i := range eps {
		eps[i] = Item{ID: uuid.Must(uuid.NewV7()), Body: "ep"}
		embs[i] = detVec(i)
	}

	clusters1 := kMeansCluster(eps, embs, 50)
	clusters2 := kMeansCluster(eps, embs, 50)

	if len(clusters1) != len(clusters2) {
		t.Fatal("different cluster count on second run (not deterministic)")
	}
}

func TestKMeans_NoEmptyClusters(t *testing.T) {
	eps := make([]Item, 6)
	embs := make([][]float32, 6)
	for i := range eps {
		eps[i] = Item{ID: uuid.Must(uuid.NewV7()), Body: "ep"}
		embs[i] = detVec(i)
	}

	clusters := kMeansCluster(eps, embs, 100)
	for i, c := range clusters {
		if len(c) == 0 {
			t.Errorf("cluster %d is empty", i)
		}
	}
}

func nearVec(base []float32, noise float32) []float32 {
	v := make([]float32, 1536)
	if len(base) > 0 {
		v[0] = base[0] + noise
	}
	if len(base) > 1 {
		v[1] = base[1] + noise
	}
	var mag float64
	for _, x := range v {
		mag += float64(x) * float64(x)
	}
	if mag > 0 {
		m := float32(math.Sqrt(mag))
		for i := range v {
			v[i] /= m
		}
	}
	return v
}

func detVec(seed int) []float32 {
	v := make([]float32, 1536)
	for i := range v {
		v[i] = float32(math.Sin(float64(seed*1000+i))) * 0.5
	}
	return v
}
