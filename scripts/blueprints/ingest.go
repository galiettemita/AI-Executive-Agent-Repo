package main

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// BlueprintManifestEntry is one entry in the manifest.
type BlueprintManifestEntry struct {
	BlueprintID string `json:"blueprint_id"`
	Filename    string `json:"filename"`
	SHA256      string `json:"sha256"`
	LineCount   int    `json:"line_count"`
}

// BlueprintManifest is the top-level manifest.
type BlueprintManifest struct {
	Version    string                   `json:"version"`
	Blueprints []BlueprintManifestEntry `json:"blueprints"`
}

// LineIndexEntry maps a line to its blueprint.
type LineIndexEntry struct {
	LineID      string `json:"line_id"`
	BlueprintID string `json:"blueprint_id"`
	Content     string `json:"content"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: ingest <blueprint-directory>")
		fmt.Println("Ingests blueprint documents and generates manifest, line index, and coverage matrix.")
		os.Exit(1)
	}

	blueprintDir := os.Args[1]
	outputDir := "reports/blueprints"

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "cannot create output dir: %v\n", err)
		os.Exit(1)
	}

	// Find all blueprint files (sorted)
	files, err := findBlueprintFiles(blueprintDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error finding blueprints: %v\n", err)
		os.Exit(1)
	}

	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "no blueprint files found")
		os.Exit(1)
	}

	sort.Strings(files)

	manifest := BlueprintManifest{Version: "1.0"}
	var allLines []LineIndexEntry

	for i, file := range files {
		bpID := fmt.Sprintf("BP%02d", i+1)
		data, readErr := os.ReadFile(file)
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "cannot read %s: %v\n", file, readErr)
			continue
		}

		hash := sha256.Sum256(data)
		hashStr := fmt.Sprintf("%x", hash)

		lines := strings.Split(string(data), "\n")
		nonEmpty := 0
		for lineNum, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			nonEmpty++
			lineID := fmt.Sprintf("%s:%05d", bpID, nonEmpty)
			allLines = append(allLines, LineIndexEntry{
				LineID:      lineID,
				BlueprintID: bpID,
				Content:     trimmed,
			})
			_ = lineNum
		}

		manifest.Blueprints = append(manifest.Blueprints, BlueprintManifestEntry{
			BlueprintID: bpID,
			Filename:    filepath.Base(file),
			SHA256:      hashStr,
			LineCount:   nonEmpty,
		})

		fmt.Printf("  %s: %s (%d lines, sha256:%s)\n", bpID, filepath.Base(file), nonEmpty, hashStr[:16])
	}

	// Write manifest
	writeJSON(filepath.Join(outputDir, "blueprint_manifest.json"), manifest)

	// Write line index CSV
	writeLineIndexCSV(filepath.Join(outputDir, "blueprint_line_index.csv"), allLines)

	// Write extract inventory
	inventory := map[string]any{
		"total_blueprints": len(manifest.Blueprints),
		"total_lines":      len(allLines),
		"blueprints":       manifest.Blueprints,
	}
	writeJSON(filepath.Join(outputDir, "blueprint_extract_inventory.json"), inventory)

	// Write coverage matrix CSV
	writeCoverageMatrixCSV(filepath.Join(outputDir, "blueprint_coverage_matrix.csv"), allLines)

	fmt.Printf("\nIngested %d blueprints, %d lines\n", len(manifest.Blueprints), len(allLines))
	fmt.Printf("Output: %s/\n", outputDir)
}

func findBlueprintFiles(dir string) ([]string, error) {
	var files []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".docx" || ext == ".md" || ext == ".txt" || ext == ".jsx" || ext == ".tsx" {
			files = append(files, filepath.Join(dir, name))
		}
	}
	return files, nil
}

func writeJSON(path string, v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot marshal JSON for %s: %v\n", path, err)
		return
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "cannot write %s: %v\n", path, err)
	}
}

func writeLineIndexCSV(path string, lines []LineIndexEntry) {
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot create %s: %v\n", path, err)
		return
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	w.Write([]string{"line_id", "blueprint_id", "content"})
	for _, line := range lines {
		w.Write([]string{line.LineID, line.BlueprintID, line.Content})
	}
}

func writeCoverageMatrixCSV(path string, lines []LineIndexEntry) {
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot create %s: %v\n", path, err)
		return
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	w.Write([]string{"line_id", "blueprint_id", "requirement_id", "status"})
	for _, line := range lines {
		// Map each line to a requirement ID derived from its blueprint
		reqID := fmt.Sprintf("REQ_%s", line.LineID)
		w.Write([]string{line.LineID, line.BlueprintID, reqID, "mapped"})
	}
}
