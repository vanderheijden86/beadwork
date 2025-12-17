package loader

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

// BeadsDirEnvVar is the name of the environment variable for custom beads directory
const BeadsDirEnvVar = "BEADS_DIR"

// GetBeadsDir returns the beads directory path, respecting BEADS_DIR env var.
// If BEADS_DIR is set, it is used directly.
// Otherwise, falls back to .beads in the given repoPath (or cwd if empty).
func GetBeadsDir(repoPath string) (string, error) {
	// Check BEADS_DIR environment variable first
	if envDir := os.Getenv(BeadsDirEnvVar); envDir != "" {
		return envDir, nil
	}

	// Fall back to .beads in repo path
	if repoPath == "" {
		var err error
		repoPath, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current working directory: %w", err)
		}
	}

	return filepath.Join(repoPath, ".beads"), nil
}

// FindJSONLPath locates the beads JSONL file in the given directory.
// Prefers issues.jsonl (canonical per beads upstream) over beads.jsonl (backward compat).
// Skips backup files and merge artifacts.
func FindJSONLPath(beadsDir string) (string, error) {
	return FindJSONLPathWithWarnings(beadsDir, nil)
}

// FindJSONLPathWithWarnings is like FindJSONLPath but optionally reports warnings
// about detected merge artifacts via the provided callback.
func FindJSONLPathWithWarnings(beadsDir string, warnFunc func(msg string)) (string, error) {
	entries, err := os.ReadDir(beadsDir)
	if err != nil {
		return "", fmt.Errorf("failed to read beads directory: %w", err)
	}

	var candidates []string
	var mergeArtifacts []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()

		// Must be a .jsonl file
		if !strings.HasSuffix(name, ".jsonl") {
			continue
		}

		// Skip backups, merge artifacts, and deletion manifests
		if strings.Contains(name, ".backup") ||
			strings.Contains(name, ".orig") ||
			strings.Contains(name, ".merge") ||
			name == "deletions.jsonl" {
			continue
		}

		// Skip git merge conflict artifacts (beads.left.jsonl, beads.right.jsonl)
		// These are OURS/THEIRS sides during a merge conflict
		if strings.HasPrefix(name, "beads.left") || strings.HasPrefix(name, "beads.right") {
			mergeArtifacts = append(mergeArtifacts, name)
			continue
		}

		candidates = append(candidates, name)
	}

	// Warn about detected merge artifacts
	if len(mergeArtifacts) > 0 && warnFunc != nil {
		warnFunc(fmt.Sprintf("Merge artifact files detected: %s. Consider running 'bd clean' to remove them.",
			strings.Join(mergeArtifacts, ", ")))
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("no beads JSONL file found in %s", beadsDir)
	}

	// Priority order for beads files per beads upstream:
	// 1. issues.jsonl (canonical - per steveyegge/beads pre-commit hook)
	// 2. beads.jsonl (backward compatibility)
	// 3. beads.base.jsonl (fallback, may be present during merge resolution)
	// 4. First candidate
	preferredNames := []string{"issues.jsonl", "beads.jsonl", "beads.base.jsonl"}

	for _, preferred := range preferredNames {
		for _, name := range candidates {
			if name == preferred {
				path := filepath.Join(beadsDir, name)
				// Check if file has content (skip empty files)
				if info, err := os.Stat(path); err == nil && info.Size() > 0 {
					return path, nil
				}
			}
		}
	}

	// Fall back to first non-empty candidate
	for _, name := range candidates {
		path := filepath.Join(beadsDir, name)
		if info, err := os.Stat(path); err == nil && info.Size() > 0 {
			return path, nil
		}
	}

	// Last resort: return first candidate even if empty
	return filepath.Join(beadsDir, candidates[0]), nil
}

// LoadIssues reads issues from the beads directory.
// Respects BEADS_DIR environment variable, otherwise uses .beads in repoPath.
// Automatically finds the correct JSONL file (issues.jsonl preferred, beads.jsonl fallback).
func LoadIssues(repoPath string) ([]model.Issue, error) {
	beadsDir, err := GetBeadsDir(repoPath)
	if err != nil {
		return nil, err
	}

	jsonlPath, err := FindJSONLPath(beadsDir)
	if err != nil {
		return nil, err
	}

	return LoadIssuesFromFile(jsonlPath)
}

// ParseOptions configures the behavior of ParseIssues.
type ParseOptions struct {
	// WarningHandler is called with warning messages (e.g., malformed JSON).
	// If nil, warnings are printed to os.Stderr.
	WarningHandler func(string)
}

// LoadIssuesFromFileWithOptions reads issues from a file with custom options.
func LoadIssuesFromFileWithOptions(path string, opts ParseOptions) ([]model.Issue, error) {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("no beads issues found at %s", path)
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open issues file: %w", err)
	}
	defer file.Close()

	return ParseIssuesWithOptions(file, opts)
}

// LoadIssuesFromFile reads issues directly from a specific JSONL file path.
func LoadIssuesFromFile(path string) ([]model.Issue, error) {
	return LoadIssuesFromFileWithOptions(path, ParseOptions{})
}

// ParseIssues parses JSONL content from a reader into issues.
// Handles UTF-8 BOM stripping, large lines, and validation.
func ParseIssues(r io.Reader) ([]model.Issue, error) {
	return ParseIssuesWithOptions(r, ParseOptions{})
}

// ParseIssuesWithOptions parses JSONL content with custom options.
func ParseIssuesWithOptions(r io.Reader, opts ParseOptions) ([]model.Issue, error) {
	var issues []model.Issue
	scanner := bufio.NewScanner(r)
	// Increase buffer size for large lines (issues can be large)
	const maxCapacity = 1024 * 1024 * 10 // 10MB
	// Start with 64KB buffer, grow up to maxCapacity
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, maxCapacity)

	// Default warning handler prints to stderr
	warn := opts.WarningHandler
	if warn == nil {
		warn = func(msg string) {
			fmt.Fprintf(os.Stderr, "Warning: %s\n", msg)
		}
	}

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Strip UTF-8 BOM if present on the first line
		if lineNum == 1 {
			line = stripBOM(line)
		}

		var issue model.Issue
		if err := json.Unmarshal(line, &issue); err != nil {
			// Skip malformed lines but warn
			warn(fmt.Sprintf("skipping malformed JSON on line %d: %v", lineNum, err))
			continue
		}

		// Validate issue
		if err := issue.Validate(); err != nil {
			// Skip invalid issues
			warn(fmt.Sprintf("skipping invalid issue on line %d: %v", lineNum, err))
			continue
		}

		issues = append(issues, issue)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading issues stream: %w", err)
	}

	return issues, nil
}

// stripBOM removes the UTF-8 Byte Order Mark if present
func stripBOM(b []byte) []byte {
	if bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}) {
		return b[3:]
	}
	return b
}
