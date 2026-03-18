package rag

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	vertexAIMultimodalModel = "multimodalembedding@001"
	vertexAIEmbeddingDim    = 1408
	vertexAIBaseURL         = "https://us-central1-aiplatform.googleapis.com/v1/projects/%s/locations/us-central1/publishers/google/models/%s:predict"
)

// MultimodalEmbedder generates 1408-dim embeddings for images and text using
// Google Vertex AI multimodalembedding@001. Both modalities share the same
// embedding space, enabling cross-modal search.
type MultimodalEmbedder struct {
	projectID  string
	httpClient *http.Client
}

// NewMultimodalEmbedder creates a multimodal embedder using GCP credentials.
func NewMultimodalEmbedder() (*MultimodalEmbedder, error) {
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		return nil, fmt.Errorf("GOOGLE_CLOUD_PROJECT not set")
	}
	ts, err := google.DefaultTokenSource(context.Background(),
		"https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return nil, fmt.Errorf("google credentials: %w", err)
	}
	hc := &http.Client{
		Transport: &oauth2Transport{base: http.DefaultTransport, tokenSource: ts},
		Timeout:   30 * time.Second,
	}
	return &MultimodalEmbedder{projectID: projectID, httpClient: hc}, nil
}

// EmbedImage generates a 1408-dim embedding for image bytes.
func (m *MultimodalEmbedder) EmbedImage(ctx context.Context, imageBytes []byte, mimeType string) ([]float32, error) {
	b64 := base64.StdEncoding.EncodeToString(imageBytes)
	return m.callVertexAI(ctx, map[string]interface{}{
		"image": map[string]string{
			"bytesBase64Encoded": b64,
			"mimeType":          mimeType,
		},
	})
}

// EmbedTextMultimodal generates a 1408-dim text embedding in the same model space as images.
func (m *MultimodalEmbedder) EmbedTextMultimodal(ctx context.Context, text string) ([]float32, error) {
	return m.callVertexAI(ctx, map[string]interface{}{"text": text})
}

func (m *MultimodalEmbedder) callVertexAI(ctx context.Context, instance map[string]interface{}) ([]float32, error) {
	url := fmt.Sprintf(vertexAIBaseURL, m.projectID, vertexAIMultimodalModel)
	reqBody := map[string]interface{}{"instances": []interface{}{instance}}
	bodyBytes, _ := json.Marshal(reqBody)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("vertex ai request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("vertex ai http: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vertex ai status %d: %s", resp.StatusCode, string(respBytes))
	}

	var result struct {
		Predictions []struct {
			ImageEmbedding []float32 `json:"imageEmbedding"`
			TextEmbedding  []float32 `json:"textEmbedding"`
		} `json:"predictions"`
	}
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, fmt.Errorf("vertex ai parse: %w", err)
	}
	if len(result.Predictions) == 0 {
		return nil, fmt.Errorf("vertex ai: empty predictions")
	}
	pred := result.Predictions[0]
	if len(pred.ImageEmbedding) > 0 {
		return pred.ImageEmbedding, nil
	}
	if len(pred.TextEmbedding) > 0 {
		return pred.TextEmbedding, nil
	}
	return nil, fmt.Errorf("vertex ai: no embedding in response")
}

// oauth2Transport injects OAuth2 Bearer tokens into HTTP requests.
type oauth2Transport struct {
	base        http.RoundTripper
	tokenSource oauth2.TokenSource
}

func (t *oauth2Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	tok, err := t.tokenSource.Token()
	if err != nil {
		return nil, err
	}
	reqCopy := req.Clone(req.Context())
	reqCopy.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	return t.base.RoundTrip(reqCopy)
}
