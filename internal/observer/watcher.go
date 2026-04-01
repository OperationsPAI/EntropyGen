package observer

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// FileChangeEvent represents a file system change notification.
type FileChangeEvent struct {
	Path   string `json:"path"`
	Action string `json:"action"` // "created", "modified", "deleted"
}

// Watcher monitors directories via fsnotify and sends change events.
type Watcher struct {
	workspaceDir string
	events       chan FileChangeEvent
}

// NewWatcher creates a file system watcher for the given workspace directory.
func NewWatcher(workspaceDir string) *Watcher {
	return &Watcher{
		workspaceDir: workspaceDir,
		events:       make(chan FileChangeEvent, 256),
	}
}

// Events returns the channel of file change events.
func (w *Watcher) Events() <-chan FileChangeEvent {
	return w.events
}

// Run starts the fsnotify watchers and blocks until ctx is cancelled.
func (w *Watcher) Run(ctx context.Context) error {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer fsw.Close()

	// Watch workspace directory recursively
	if err := addDirRecursive(fsw, w.workspaceDir); err != nil {
		slog.Warn("watcher: cannot watch workspace dir", "dir", w.workspaceDir, "err", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-fsw.Events:
			if !ok {
				return nil
			}
			w.handleEvent(event)
			// Watch newly created directories
			if event.Has(fsnotify.Create) {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					_ = fsw.Add(event.Name)
				}
			}
		case err, ok := <-fsw.Errors:
			if !ok {
				return nil
			}
			slog.Warn("watcher: fsnotify error", "err", err)
		}
	}
}

// handleEvent converts an fsnotify event into a FileChangeEvent.
func (w *Watcher) handleEvent(event fsnotify.Event) {
	path := event.Name

	// Workspace file change
	if strings.HasPrefix(path, w.workspaceDir) {
		relPath, err := filepath.Rel(w.workspaceDir, path)
		if err != nil {
			return
		}

		// Skip hidden files/dirs
		for _, part := range strings.Split(relPath, string(filepath.Separator)) {
			if strings.HasPrefix(part, ".") {
				return
			}
		}

		var action string
		switch {
		case event.Has(fsnotify.Create):
			action = "created"
		case event.Has(fsnotify.Write):
			action = "modified"
		case event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename):
			action = "deleted"
		default:
			return
		}

		select {
		case w.events <- FileChangeEvent{Path: relPath, Action: action}:
		default:
		}
	}
}

// addDirRecursive adds a directory and all subdirectories to the watcher.
func addDirRecursive(fsw *fsnotify.Watcher, dir string) error {
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if d.IsDir() {
			// Skip hidden directories
			if strings.HasPrefix(d.Name(), ".") && path != dir {
				return filepath.SkipDir
			}
			return fsw.Add(path)
		}
		return nil
	})
}
