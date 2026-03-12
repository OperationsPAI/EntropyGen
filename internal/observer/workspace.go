package observer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FileNode represents a file or directory in the workspace tree.
type FileNode struct {
	Name     string     `json:"name"`
	Type     string     `json:"type"` // "file" or "dir"
	Modified bool       `json:"modified"`
	Children []FileNode `json:"children,omitempty"`
}

// FileContent represents the content of a single file.
type FileContent struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Language string `json:"language"`
}

// DiffResult represents the output of git diff.
type DiffResult struct {
	Diff         string `json:"diff"`
	FilesChanged int    `json:"files_changed"`
	Insertions   int    `json:"insertions"`
	Deletions    int    `json:"deletions"`
}

// BuildFileTree walks the workspace directory and builds a recursive tree.
// It uses git status to mark modified/new files.
func BuildFileTree(workDir string) (FileNode, error) {
	modified := gitModifiedFiles(workDir)

	root := FileNode{
		Name: filepath.Base(workDir),
		Type: "dir",
	}

	root.Children = walkDir(workDir, workDir, modified)
	return root, nil
}

// walkDir recursively builds the tree, skipping hidden directories.
func walkDir(baseDir, currentDir string, modified map[string]bool) []FileNode {
	entries, err := os.ReadDir(currentDir)
	if err != nil {
		return nil
	}

	var nodes []FileNode
	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden files/dirs
		if strings.HasPrefix(name, ".") {
			continue
		}

		fullPath := filepath.Join(currentDir, name)
		relPath, _ := filepath.Rel(baseDir, fullPath)

		if entry.IsDir() {
			children := walkDir(baseDir, fullPath, modified)
			// Check if any child is modified
			dirMod := false
			for _, c := range children {
				if c.Modified {
					dirMod = true
					break
				}
			}
			nodes = append(nodes, FileNode{
				Name:     name,
				Type:     "dir",
				Modified: dirMod,
				Children: children,
			})
		} else {
			nodes = append(nodes, FileNode{
				Name:     name,
				Type:     "file",
				Modified: modified[relPath],
			})
		}
	}

	return nodes
}

// gitModifiedFiles runs git status --porcelain and returns a set of modified file paths.
func gitModifiedFiles(workDir string) map[string]bool {
	result := make(map[string]bool)
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = workDir
	out, err := cmd.Output()
	if err != nil {
		return result
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if len(line) < 4 {
			continue
		}
		// Format: "XY filename" or "XY filename -> renamed"
		file := strings.TrimSpace(line[3:])
		if idx := strings.Index(file, " -> "); idx >= 0 {
			file = file[idx+4:]
		}
		result[file] = true
	}
	return result
}

// ReadFile reads a file from the workspace, validating against path traversal.
// baseDir is the allowed root directory (e.g., OPENCLAW_HOME).
func ReadFile(baseDir, requestedPath string) (FileContent, error) {
	if err := ValidatePath(baseDir, requestedPath); err != nil {
		return FileContent{}, err
	}

	absPath := filepath.Join(baseDir, requestedPath)
	data, err := os.ReadFile(absPath)
	if err != nil {
		return FileContent{}, err
	}

	return FileContent{
		Path:     requestedPath,
		Content:  string(data),
		Language: inferLanguage(filepath.Ext(requestedPath)),
	}, nil
}

// ValidatePath checks that the requested path does not escape the base directory.
func ValidatePath(baseDir, requestedPath string) error {
	// Reject absolute paths immediately
	if filepath.IsAbs(requestedPath) {
		return fmt.Errorf("path traversal denied: %s", requestedPath)
	}

	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return fmt.Errorf("invalid base directory: %w", err)
	}

	joined := filepath.Join(absBase, requestedPath)
	resolved, err := filepath.EvalSymlinks(filepath.Dir(joined))
	if err != nil {
		// Parent dir may not exist yet for the file itself; try the path as-is
		resolved = filepath.Clean(joined)
	} else {
		resolved = filepath.Join(resolved, filepath.Base(joined))
	}

	if !strings.HasPrefix(resolved, absBase) {
		return fmt.Errorf("path traversal denied: %s", requestedPath)
	}
	return nil
}

// GetGitDiff runs git diff in the workspace directory.
func GetGitDiff(workDir string) DiffResult {
	// Get the diff content
	cmd := exec.Command("git", "diff")
	cmd.Dir = workDir
	diffOut, err := cmd.Output()
	if err != nil {
		return DiffResult{}
	}

	// Get the stat summary
	statCmd := exec.Command("git", "diff", "--stat")
	statCmd.Dir = workDir
	statOut, _ := statCmd.Output()

	filesChanged, insertions, deletions := parseDiffStat(string(statOut))

	return DiffResult{
		Diff:         string(diffOut),
		FilesChanged: filesChanged,
		Insertions:   insertions,
		Deletions:    deletions,
	}
}

// parseDiffStat parses the last line of git diff --stat output.
// Example: " 3 files changed, 10 insertions(+), 5 deletions(-)"
func parseDiffStat(stat string) (files, ins, dels int) {
	lines := strings.Split(strings.TrimSpace(stat), "\n")
	if len(lines) == 0 {
		return 0, 0, 0
	}
	summary := lines[len(lines)-1]

	fmt.Sscanf(extractNumber(summary, "file"), "%d", &files)
	fmt.Sscanf(extractNumber(summary, "insertion"), "%d", &ins)
	fmt.Sscanf(extractNumber(summary, "deletion"), "%d", &dels)
	return
}

// extractNumber finds a number before a keyword in a string.
func extractNumber(s, keyword string) string {
	idx := strings.Index(s, keyword)
	if idx < 0 {
		return "0"
	}
	// Walk backwards to find the number
	end := idx - 1
	for end >= 0 && s[end] == ' ' {
		end--
	}
	start := end
	for start >= 0 && s[start] >= '0' && s[start] <= '9' {
		start--
	}
	if start == end {
		return "0"
	}
	return s[start+1 : end+1]
}

// inferLanguage maps file extensions to language identifiers.
func inferLanguage(ext string) string {
	languages := map[string]string{
		".go":    "go",
		".py":    "python",
		".js":    "javascript",
		".ts":    "typescript",
		".tsx":   "typescriptreact",
		".jsx":   "javascriptreact",
		".json":  "json",
		".yaml":  "yaml",
		".yml":   "yaml",
		".md":    "markdown",
		".sh":    "shell",
		".bash":  "shell",
		".css":   "css",
		".html":  "html",
		".sql":   "sql",
		".rs":    "rust",
		".java":  "java",
		".rb":    "ruby",
		".toml":  "toml",
		".xml":   "xml",
		".c":     "c",
		".cpp":   "cpp",
		".h":     "c",
		".hpp":   "cpp",
		".proto": "protobuf",
	}
	if lang, ok := languages[ext]; ok {
		return lang
	}
	return "plaintext"
}
