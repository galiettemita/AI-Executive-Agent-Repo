// scripts/blueprints/ingest.go
// Blueprint Ingestion + Coverage Matrix generator (T01).
// Reads .docx and .jsx files from extracted-blueprints/, assigns BP IDs,
// and produces deterministic manifest, line index, extract inventory,
// and coverage matrix reports.
package main

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ManifestEntry represents one blueprint in the manifest.
type ManifestEntry struct {
	BlueprintID string `json:"blueprint_id"`
	Filename    string `json:"filename"`
	SHA256      string `json:"sha256"`
	LineCount   int    `json:"line_count"`
	Path        string `json:"path"`
}

// ExtractEntry represents an extract from a blueprint.
type ExtractEntry struct {
	ExtractID      string `json:"extract_id"`
	Type           string `json:"type"`
	ContentPreview string `json:"content_preview"`
}

// InventoryEntry represents a blueprint's extracts in the inventory.
type InventoryEntry struct {
	BlueprintID string         `json:"blueprint_id"`
	Extracts    []ExtractEntry `json:"extracts"`
}

func main() {
	repoRoot := findRepoRoot()
	blueprintsDir := filepath.Join(repoRoot, "extracted-blueprints")
	reportsDir := filepath.Join(repoRoot, "reports", "blueprints")

	if err := os.MkdirAll(reportsDir, 0o755); err != nil {
		fatal("creating reports dir: %v", err)
	}

	// Collect the 17 original files (.docx and .jsx), sorted stably.
	entries, err := os.ReadDir(blueprintsDir)
	if err != nil {
		fatal("reading blueprints dir: %v", err)
	}

	var originals []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext == ".docx" || ext == ".jsx" {
			originals = append(originals, e.Name())
		}
	}
	sort.Strings(originals)

	if len(originals) != 17 {
		fatal("expected 17 blueprint files, found %d", len(originals))
	}

	var manifest []ManifestEntry
	var inventory []InventoryEntry
	var lineIndexRows [][]string
	var coverageRows [][]string

	for i, filename := range originals {
		bpID := fmt.Sprintf("BP%02d", i+1)
		ext := strings.ToLower(filepath.Ext(filename))
		origPath := filepath.Join(blueprintsDir, filename)

		// SHA-256 of the original file.
		hash, err := fileSHA256(origPath)
		if err != nil {
			fatal("hashing %s: %v", filename, err)
		}

		// Parse lines from content source.
		var lines []string
		if ext == ".docx" {
			// Read the .txt sibling.
			txtName := strings.TrimSuffix(filename, filepath.Ext(filename)) + ".txt"
			txtPath := filepath.Join(blueprintsDir, txtName)
			lines, err = readNonEmptyLines(txtPath)
			if err != nil {
				fatal("reading txt for %s: %v", filename, err)
			}
		} else {
			// .jsx -- read directly, non-empty lines.
			lines, err = readNonEmptyLines(origPath)
			if err != nil {
				fatal("reading jsx %s: %v", filename, err)
			}
		}

		manifest = append(manifest, ManifestEntry{
			BlueprintID: bpID,
			Filename:    filename,
			SHA256:      hash,
			LineCount:   len(lines),
			Path:        "extracted-blueprints/" + filename,
		})

		// Build extracts -- treat each non-empty line as an extract.
		var extracts []ExtractEntry
		for j, line := range lines {
			lineNum := j + 1
			lineID := fmt.Sprintf("%s:%05d", bpID, lineNum)
			preview := truncate(line, 200)

			lineIndexRows = append(lineIndexRows, []string{
				lineID, bpID, fmt.Sprintf("%d", lineNum), preview,
			})

			coverageRows = append(coverageRows, []string{
				lineID, bpID, "UNMAPPED",
			})

			extractID := fmt.Sprintf("%s:E%05d", bpID, lineNum)
			extractType := "paragraph"
			if ext == ".jsx" {
				extractType = "code_line"
			}
			extracts = append(extracts, ExtractEntry{
				ExtractID:      extractID,
				Type:           extractType,
				ContentPreview: preview,
			})
		}

		inventory = append(inventory, InventoryEntry{
			BlueprintID: bpID,
			Extracts:    extracts,
		})

		fmt.Printf("  %s: %s (%d lines)\n", bpID, filename, len(lines))
	}

	// Write manifest JSON.
	writeJSON(filepath.Join(reportsDir, "blueprint_manifest.json"), manifest)

	// Write line index CSV.
	writeCSV(filepath.Join(reportsDir, "blueprint_line_index.csv"),
		[]string{"line_id", "blueprint_id", "line_number", "content"},
		lineIndexRows,
	)

	// Write extract inventory JSON.
	writeJSON(filepath.Join(reportsDir, "blueprint_extract_inventory.json"), inventory)

	// Write coverage matrix CSV.
	writeCSV(filepath.Join(reportsDir, "blueprint_coverage_matrix.csv"),
		[]string{"line_id", "blueprint_id", "requirement_id"},
		coverageRows,
	)

	// Write line index JSONL.
	writeLineIndexJSONL(filepath.Join(reportsDir, "blueprint_line_index.jsonl"), lineIndexRows)

	fmt.Printf("\nIngested %d blueprints, %d lines total\n", len(manifest), len(lineIndexRows))
	fmt.Printf("Reports written to %s\n", reportsDir)
}

// findRepoRoot walks up from cwd to find the directory containing go.mod.
func findRepoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		fatal("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			fatal("could not find repo root (no go.mod found)")
		}
		dir = parent
	}
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// readNonEmptyLines reads a text file and returns all non-empty lines.
func readNonEmptyLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	raw := strings.Split(string(data), "\n")
	var lines []string
	for _, l := range raw {
		l = strings.TrimRight(l, "\r")
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	return lines, nil
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}

func writeJSON(path string, v interface{}) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fatal("marshal json: %v", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		fatal("writing %s: %v", path, err)
	}
}

func writeCSV(path string, header []string, rows [][]string) {
	f, err := os.Create(path)
	if err != nil {
		fatal("creating %s: %v", path, err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write(header); err != nil {
		fatal("writing csv header: %v", err)
	}
	for _, row := range rows {
		if err := w.Write(row); err != nil {
			fatal("writing csv row: %v", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		fatal("csv flush: %v", err)
	}
}

// LineIndexJSONLEntry is one record in the JSONL output.
type LineIndexJSONLEntry struct {
	LineID      string `json:"line_id"`
	BlueprintID string `json:"blueprint_id"`
	LineNumber  string `json:"line_number"`
	Content     string `json:"content"`
}

func writeLineIndexJSONL(path string, rows [][]string) {
	f, err := os.Create(path)
	if err != nil {
		fatal("creating %s: %v", path, err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	for _, row := range rows {
		entry := LineIndexJSONLEntry{
			LineID:      row[0],
			BlueprintID: row[1],
			LineNumber:  row[2],
			Content:     row[3],
		}
		if err := enc.Encode(entry); err != nil {
			fatal("writing jsonl row: %v", err)
		}
	}
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "FATAL: "+format+"\n", args...)
	os.Exit(1)
}
