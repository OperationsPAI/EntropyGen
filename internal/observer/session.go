package observer

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SessionInfo holds metadata about a JSONL session file.
type SessionInfo struct {
	ID           string `json:"id"`
	StartedAt    string `json:"started_at"`
	MessageCount int    `json:"message_count"`
	IsCurrent    bool   `json:"is_current"`
	Filename     string `json:"filename"`
}

// sessionFirstLine represents the minimal structure of a JSONL first line.
type sessionFirstLine struct {
	SessionID string `json:"sessionId"`
	Timestamp string `json:"timestamp"`
}

// ListSessions reads all .jsonl files in the completions directory and returns metadata.
func ListSessions(completionsDir string) ([]SessionInfo, error) {
	entries, err := os.ReadDir(completionsDir)
	if err != nil {
		return nil, err
	}

	var sessions []SessionInfo
	var latestMod time.Time
	latestIdx := -1

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		fullPath := filepath.Join(completionsDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		si := SessionInfo{
			Filename: entry.Name(),
		}

		// Parse first line for session metadata
		if first, lineCount, parseErr := parseSessionFile(fullPath); parseErr == nil {
			si.ID = first.SessionID
			si.StartedAt = first.Timestamp
			si.MessageCount = lineCount
		} else {
			// Fallback: use filename as ID
			si.ID = strings.TrimSuffix(entry.Name(), ".jsonl")
			si.StartedAt = info.ModTime().UTC().Format(time.RFC3339)
		}

		if info.ModTime().After(latestMod) {
			latestMod = info.ModTime()
			latestIdx = len(sessions)
		}

		sessions = append(sessions, si)
	}

	if latestIdx >= 0 && latestIdx < len(sessions) {
		sessions[latestIdx].IsCurrent = true
	}

	// Sort by started_at descending
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartedAt > sessions[j].StartedAt
	})

	return sessions, nil
}

// parseSessionFile reads the first line for metadata and counts total lines.
func parseSessionFile(path string) (sessionFirstLine, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return sessionFirstLine{}, 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)

	var first sessionFirstLine
	lineCount := 0

	for scanner.Scan() {
		lineCount++
		if lineCount == 1 {
			_ = json.Unmarshal(scanner.Bytes(), &first)
		}
	}

	return first, lineCount, scanner.Err()
}

// ReadSessionContent reads the full content of a session JSONL file and returns raw lines.
func ReadSessionContent(completionsDir, sessionID string) ([]json.RawMessage, error) {
	path, err := findSessionFile(completionsDir, sessionID)
	if err != nil {
		return nil, err
	}
	return readJSONLLines(path)
}

// ReadCurrentSession reads the most recently modified JSONL file.
func ReadCurrentSession(completionsDir string) ([]json.RawMessage, string, error) {
	path, err := findCurrentSessionFile(completionsDir)
	if err != nil {
		return nil, "", err
	}

	lines, err := readJSONLLines(path)
	if err != nil {
		return nil, "", err
	}

	return lines, filepath.Base(path), nil
}

// findSessionFile locates a session file by ID (matching filename or parsed sessionId).
func findSessionFile(completionsDir, sessionID string) (string, error) {
	// Try direct filename match first
	candidate := filepath.Join(completionsDir, sessionID+".jsonl")
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}

	// Scan all files for matching sessionId in first line
	entries, err := os.ReadDir(completionsDir)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		fullPath := filepath.Join(completionsDir, entry.Name())
		first, _, parseErr := parseSessionFile(fullPath)
		if parseErr == nil && first.SessionID == sessionID {
			return fullPath, nil
		}
	}

	return "", os.ErrNotExist
}

// findCurrentSessionFile returns the path to the most recently modified .jsonl file.
func findCurrentSessionFile(completionsDir string) (string, error) {
	entries, err := os.ReadDir(completionsDir)
	if err != nil {
		return "", err
	}

	var latestPath string
	var latestMod time.Time

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(latestMod) {
			latestMod = info.ModTime()
			latestPath = filepath.Join(completionsDir, entry.Name())
		}
	}

	if latestPath == "" {
		return "", os.ErrNotExist
	}
	return latestPath, nil
}

// readJSONLLines reads all lines from a JSONL file as raw JSON.
func readJSONLLines(path string) ([]json.RawMessage, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []json.RawMessage
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		cp := make([]byte, len(line))
		copy(cp, line)
		lines = append(lines, json.RawMessage(cp))
	}
	return lines, scanner.Err()
}
