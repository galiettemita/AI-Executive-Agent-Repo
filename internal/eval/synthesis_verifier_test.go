package eval_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brevio/brevio/internal/eval"
)

type mockVerifierLLM struct {
	callResponses [][]string // responses per call
	callIndex     int
	err           error
}

func (m *mockVerifierLLM) SampleMultiple(_ context.Context, _, _, _, _ string, n int) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIndex >= len(m.callResponses) {
		return nil, fmt.Errorf("no more mock responses")
	}
	resp := m.callResponses[m.callIndex]
	m.callIndex++
	return resp, nil
}

func TestSynthesisVerifier_PassesFastPathForT0(t *testing.T) {
	sv := eval.NewSynthesisVerifier(&mockVerifierLLM{callResponses: [][]string{}}, 0.7)
	result, err := sv.Verify(context.Background(), "ws-1", "sys", "user", "response", "t0")
	require.NoError(t, err)
	assert.True(t, result.Passed)
	assert.Equal(t, 1.0, result.ConsistencyScore)
}

func TestSynthesisVerifier_PassesFastPathForT1(t *testing.T) {
	sv := eval.NewSynthesisVerifier(&mockVerifierLLM{callResponses: [][]string{}}, 0.7)
	result, err := sv.Verify(context.Background(), "ws-1", "sys", "user", "response", "t1")
	require.NoError(t, err)
	assert.True(t, result.Passed)
}

func TestSynthesisVerifier_FlagsInconsistentT2Response(t *testing.T) {
	llm := &mockVerifierLLM{
		callResponses: [][]string{
			{"different response A", "different response B"},
			{`{"consistency_score": 0.3, "flagged_claims": ["date mismatch"], "reasoning": "inconsistent dates"}`},
		},
	}
	sv := eval.NewSynthesisVerifier(llm, 0.7)
	result, err := sv.Verify(context.Background(), "ws-1", "sys", "user", "primary", "t2")
	require.NoError(t, err)
	assert.False(t, result.Passed)
	assert.Less(t, result.ConsistencyScore, 0.7)
}

func TestSynthesisVerifier_PassesConsistentT2Response(t *testing.T) {
	llm := &mockVerifierLLM{
		callResponses: [][]string{
			{"same response", "same response"},
			{`{"consistency_score": 0.95, "flagged_claims": [], "reasoning": "all consistent"}`},
		},
	}
	sv := eval.NewSynthesisVerifier(llm, 0.7)
	result, err := sv.Verify(context.Background(), "ws-1", "sys", "user", "same response", "t2")
	require.NoError(t, err)
	assert.True(t, result.Passed)
	assert.GreaterOrEqual(t, result.ConsistencyScore, 0.7)
}

func TestSynthesisVerifier_HandlesSamplerError(t *testing.T) {
	llm := &mockVerifierLLM{err: fmt.Errorf("LLM unavailable")}
	sv := eval.NewSynthesisVerifier(llm, 0.7)
	result, err := sv.Verify(context.Background(), "ws-1", "sys", "user", "primary", "t2")
	require.NoError(t, err)
	assert.True(t, result.Passed, "should pass gracefully on LLM error")
}

func TestParseVerificationResult_ExtractsScore(t *testing.T) {
	raw := `{"consistency_score": 0.42, "flagged_claims": ["claim1"], "reasoning": "low overlap"}`
	result := eval.ParseVerificationResult(raw)
	assert.InDelta(t, 0.42, result.ConsistencyScore, 0.01)
	assert.Equal(t, "low overlap", result.Reasoning)
}
