package rag

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"math"
)

// MockEmbeddingProvider returns deterministic unit-normalized vectors derived
// from SHA-256 of the input text.
type MockEmbeddingProvider struct {
	dims  int
	model string
}

// NewMockEmbeddingProvider creates a mock with the given dimensionality.
// Use dims=1536 to match the production schema.
func NewMockEmbeddingProvider(dims int) *MockEmbeddingProvider {
	if dims <= 0 {
		dims = 1536
	}
	return &MockEmbeddingProvider{dims: dims, model: "mock-embedding-v1"}
}

func (m *MockEmbeddingProvider) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, text := range texts {
		out[i] = m.deriveVector(text)
	}
	return out, nil
}

func (m *MockEmbeddingProvider) Dimensions() int    { return m.dims }
func (m *MockEmbeddingProvider) ModelName() string   { return m.model }

func (m *MockEmbeddingProvider) deriveVector(text string) []float32 {
	seed := sha256.Sum256([]byte(text))
	vec := make([]float32, m.dims)

	for i := 0; i < m.dims; i++ {
		indexBuf := [4]byte{}
		binary.LittleEndian.PutUint32(indexBuf[:], uint32(i))
		expanded := sha256.Sum256(append(seed[:], indexBuf[:]...))
		bits := binary.LittleEndian.Uint32(expanded[:4])
		vec[i] = float32(int32(bits)) / float32(math.MaxInt32)
	}

	return l2Normalize(vec)
}

func l2Normalize(v []float32) []float32 {
	var sumSq float64
	for _, x := range v {
		sumSq += float64(x) * float64(x)
	}
	if sumSq == 0 {
		return v
	}
	mag := float32(math.Sqrt(sumSq))
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = x / mag
	}
	return out
}
