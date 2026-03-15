package golden

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// GoldenQuery is a single entry in the golden evaluation dataset.
type GoldenQuery struct {
	ID               string    `json:"id"`
	Description      string    `json:"description"`
	Query            string    `json:"query"`
	ExpectedChunkIDs []string  `json:"expected_chunk_ids"`
	ExpectedAnswer   string    `json:"expected_answer"`
	WorkspaceID      string    `json:"workspace_id"`
	Category         string    `json:"category"`
	Difficulty       string    `json:"difficulty"`
	CreatedAt        time.Time `json:"created_at"`
	Notes            string    `json:"notes,omitempty"`
}

// GoldenDataset is the full collection of golden evaluation queries.
type GoldenDataset struct {
	Version     string        `json:"version"`
	Description string        `json:"description"`
	CreatedAt   time.Time     `json:"created_at"`
	Queries     []GoldenQuery `json:"queries"`
}

// Load reads a GoldenDataset from a JSON file.
func Load(path string) (*GoldenDataset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("golden dataset: read file: %w", err)
	}
	var ds GoldenDataset
	if err := json.Unmarshal(data, &ds); err != nil {
		return nil, fmt.Errorf("golden dataset: unmarshal: %w", err)
	}
	if len(ds.Queries) == 0 {
		return nil, fmt.Errorf("golden dataset: no queries found in %s", path)
	}
	return &ds, nil
}

// Save writes a GoldenDataset to a JSON file.
func Save(ds *GoldenDataset, path string) error {
	data, err := json.MarshalIndent(ds, "", "  ")
	if err != nil {
		return fmt.Errorf("golden dataset: marshal: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// Filter returns queries matching the given category and/or difficulty.
func (ds *GoldenDataset) Filter(category, difficulty string) []GoldenQuery {
	var result []GoldenQuery
	for _, q := range ds.Queries {
		if category != "" && q.Category != category {
			continue
		}
		if difficulty != "" && q.Difficulty != difficulty {
			continue
		}
		result = append(result, q)
	}
	return result
}
