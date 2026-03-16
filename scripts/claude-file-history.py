#!/usr/bin/env python3
"""
claude-file-history: Recover file snapshots from Claude Code conversation logs.

Claude Code stores:
  1. ~/.claude/file-history/<session>/<hash>@v<N>  — pre-edit snapshots (full file content)
  2. JSONL tool_use(Write) — full file content for new files
  3. JSONL tool_use(Edit)  — old_string → new_string diffs

This script replays those operations to reconstruct a file's state at any point in time.

Usage:
    # List all files touched across all sessions for this project
    python3 scripts/claude-file-history.py list

    # Show timeline of edits for a specific file
    python3 scripts/claude-file-history.py timeline <file-path>

    # Output the reconstructed file content at a given timestamp
    python3 scripts/claude-file-history.py snapshot <file-path> <timestamp-or-index>

    # Diff the snapshot against the current file on disk
    python3 scripts/claude-file-history.py diff <file-path> <timestamp-or-index>

Timestamps can be ISO format (2026-03-16T08:24:00Z) or a 0-based index from `timeline`.
"""

import argparse
import difflib
import json
import os
import sys
from collections import defaultdict
from dataclasses import dataclass, field
from datetime import datetime
from pathlib import Path
from typing import Optional


# --- Configuration ---

CLAUDE_DIR = Path.home() / ".claude"
FILE_HISTORY_DIR = CLAUDE_DIR / "file-history"
REPO_ROOT = Path.cwd()  # Assumed to be the repo root


def normalize_file_path(file_path: str) -> str:
    """Normalize to absolute path so snapshots (relative) and edits (absolute) merge."""
    if os.path.isabs(file_path):
        return file_path
    return str(REPO_ROOT / file_path)


# Auto-detect project dir from CWD
def detect_project_dir() -> Path:
    cwd = Path.cwd()
    slug = str(cwd).replace("/", "-").lstrip("-")
    candidate = CLAUDE_DIR / "projects" / slug
    if candidate.exists():
        return candidate
    # Fallback: search for matching prefix
    projects_dir = CLAUDE_DIR / "projects"
    if projects_dir.exists():
        for d in projects_dir.iterdir():
            if slug in d.name:
                return d
    print(f"Error: Cannot find project dir for {cwd}", file=sys.stderr)
    print(f"  Tried: {candidate}", file=sys.stderr)
    sys.exit(1)


# --- Data model ---

@dataclass
class FileOp:
    """A single file operation extracted from conversation logs."""
    timestamp: str           # ISO timestamp
    session_id: str
    op_type: str             # "write", "edit", "snapshot_v1"
    file_path: str           # Absolute path
    # For write:
    content: Optional[str] = None
    # For edit:
    old_string: Optional[str] = None
    new_string: Optional[str] = None
    # For snapshot:
    backup_file: Optional[str] = None
    version: int = 0
    # Was it successful?
    success: bool = True
    # Source description
    source: str = ""

    @property
    def dt(self) -> datetime:
        ts = self.timestamp.rstrip("Z")
        if "." in ts:
            return datetime.fromisoformat(ts)
        return datetime.fromisoformat(ts)


@dataclass
class FileTimeline:
    """Complete timeline of operations on a single file."""
    file_path: str
    ops: list = field(default_factory=list)

    def add(self, op: FileOp):
        self.ops.append(op)
        self.ops.sort(key=lambda o: o.timestamp)


# --- Extraction ---

def extract_ops_from_jsonl(jsonl_path: Path, session_id: str) -> list[FileOp]:
    """Extract Write/Edit operations from a JSONL conversation log."""
    ops = []
    tool_use_map = {}  # id -> FileOp (pending success check)
    tool_timestamps = {}  # tool_use id -> timestamp from the message

    try:
        with open(jsonl_path) as f:
            for line in f:
                try:
                    obj = json.loads(line)
                except json.JSONDecodeError:
                    continue

                msg = obj.get("message", {})
                content = msg.get("content", [])
                msg_ts = obj.get("timestamp", "")

                if not isinstance(content, list):
                    continue

                for item in content:
                    if not isinstance(item, dict):
                        continue

                    if item.get("type") == "tool_use":
                        name = item.get("name", "")
                        inp = item.get("input", {})
                        tool_id = item.get("id", "")

                        if name == "Write":
                            file_path = inp.get("file_path", "")
                            file_content = inp.get("content", "")
                            if file_path and file_content:
                                op = FileOp(
                                    timestamp=msg_ts,
                                    session_id=session_id,
                                    op_type="write",
                                    file_path=file_path,
                                    content=file_content,
                                    source=f"session:{session_id[:8]}",
                                )
                                tool_use_map[tool_id] = op

                        elif name == "Edit":
                            file_path = inp.get("file_path", "")
                            old_s = inp.get("old_string", "")
                            new_s = inp.get("new_string", "")
                            if file_path and (old_s or new_s):
                                op = FileOp(
                                    timestamp=msg_ts,
                                    session_id=session_id,
                                    op_type="edit",
                                    file_path=file_path,
                                    old_string=old_s,
                                    new_string=new_s,
                                    source=f"session:{session_id[:8]}",
                                )
                                tool_use_map[tool_id] = op

                    elif item.get("type") == "tool_result":
                        tool_id = item.get("tool_use_id", "")
                        is_error = item.get("is_error", False)
                        result_text = ""
                        rc = item.get("content", "")
                        if isinstance(rc, list):
                            for r in rc:
                                if isinstance(r, dict) and r.get("type") == "text":
                                    result_text += r.get("text", "")
                        elif isinstance(rc, str):
                            result_text = rc

                        if tool_id in tool_use_map:
                            op = tool_use_map[tool_id]
                            if is_error or "tool_use_error" in result_text:
                                op.success = False
                            else:
                                op.success = True

    except Exception as e:
        print(f"Warning: Failed to read {jsonl_path}: {e}", file=sys.stderr)

    # Only return successful ops
    for op in tool_use_map.values():
        if op.success:
            ops.append(op)

    return ops


def extract_snapshots_from_jsonl(jsonl_path: Path, session_id: str) -> list[FileOp]:
    """Extract file-history-snapshot entries from JSONL."""
    ops = []
    # Track the highest version seen per file path
    seen = {}  # (file_path, version) -> FileOp

    try:
        with open(jsonl_path) as f:
            for line in f:
                try:
                    obj = json.loads(line)
                except json.JSONDecodeError:
                    continue

                if obj.get("type") != "file-history-snapshot":
                    continue

                snap = obj.get("snapshot", {})
                for file_path, info in snap.get("trackedFileBackups", {}).items():
                    if not isinstance(info, dict):
                        continue
                    backup_name = info.get("backupFileName", "")
                    version = info.get("version", 1)
                    backup_time = info.get("backupTime", "")

                    key = (file_path, version)
                    if key not in seen:
                        op = FileOp(
                            timestamp=backup_time,
                            session_id=session_id,
                            op_type=f"snapshot_v{version}",
                            file_path=file_path,
                            backup_file=backup_name,
                            version=version,
                            source=f"file-history:{session_id[:8]}",
                        )
                        seen[key] = op

    except Exception as e:
        print(f"Warning: Failed to read snapshots from {jsonl_path}: {e}", file=sys.stderr)

    return list(seen.values())


def read_backup_file(session_id: str, backup_filename: str) -> Optional[str]:
    """Read a backup file from file-history storage."""
    path = FILE_HISTORY_DIR / session_id / backup_filename
    if path.exists():
        return path.read_text(errors="replace")
    # Search all sessions (backup files may be shared via hash)
    for session_dir in FILE_HISTORY_DIR.iterdir():
        candidate = session_dir / backup_filename
        if candidate.exists():
            return candidate.read_text(errors="replace")
    return None


# --- Reconstruction ---

def apply_edit(content: str, old_string: str, new_string: str) -> str:
    """Apply a single Edit operation to file content."""
    if old_string in content:
        return content.replace(old_string, new_string, 1)
    # If exact match fails, the edit may have been against different content
    return content


def reconstruct_at_index(timeline: FileTimeline, target_idx: int) -> Optional[str]:
    """Reconstruct file content after applying operations up to target_idx (inclusive).

    Strategy:
    1. Find the latest snapshot_v<N> at or before target_idx as baseline.
    2. If a Write op exists at or before target_idx, use the latest one as baseline.
    3. Apply all Edit ops between the baseline and target_idx.
    """
    if target_idx < 0 or target_idx >= len(timeline.ops):
        return None

    # Find best starting point
    baseline_content = None
    baseline_idx = -1

    for i in range(target_idx + 1):
        op = timeline.ops[i]
        if op.op_type == "write" and op.content:
            baseline_content = op.content
            baseline_idx = i
        elif op.op_type.startswith("snapshot_v") and op.backup_file:
            content = read_backup_file(op.session_id, op.backup_file)
            if content is not None:
                baseline_content = content
                baseline_idx = i

    if baseline_content is None:
        # No baseline found; try reading from the earliest snapshot as pre-edit state
        # and replay all edits
        for op in timeline.ops:
            if op.op_type.startswith("snapshot_v") and op.backup_file:
                content = read_backup_file(op.session_id, op.backup_file)
                if content is not None:
                    baseline_content = content
                    baseline_idx = -1  # start from beginning
                    break

    if baseline_content is None:
        return None

    # Apply edits from baseline to target
    content = baseline_content
    start = baseline_idx + 1
    for i in range(start, target_idx + 1):
        op = timeline.ops[i]
        if op.op_type == "edit" and op.old_string is not None:
            content = apply_edit(content, op.old_string, op.new_string or "")
        elif op.op_type == "write" and op.content:
            content = op.content

    return content


# --- Main logic ---

def collect_all_timelines(project_dir: Path) -> dict[str, FileTimeline]:
    """Scan all sessions and build per-file timelines."""
    timelines: dict[str, FileTimeline] = {}

    def ensure_timeline(path: str) -> FileTimeline:
        path = normalize_file_path(path)
        if path not in timelines:
            timelines[path] = FileTimeline(file_path=path)
        return timelines[path]

    # Collect all JSONL files (main sessions + subagents)
    jsonl_files = []
    for f in project_dir.glob("*.jsonl"):
        session_id = f.stem
        jsonl_files.append((f, session_id))

    for session_dir in project_dir.iterdir():
        if session_dir.is_dir() and (session_dir / "subagents").exists():
            for sf in (session_dir / "subagents").glob("*.jsonl"):
                jsonl_files.append((sf, session_dir.name))

    total = len(jsonl_files)
    for idx, (jsonl_path, session_id) in enumerate(jsonl_files):
        print(f"\r  Scanning [{idx+1}/{total}] {jsonl_path.name[:40]}...", end="", file=sys.stderr)

        # Extract tool_use ops
        ops = extract_ops_from_jsonl(jsonl_path, session_id)
        for op in ops:
            ensure_timeline(op.file_path).add(op)

        # Extract snapshots
        snaps = extract_snapshots_from_jsonl(jsonl_path, session_id)
        for op in snaps:
            ensure_timeline(op.file_path).add(op)

    print("", file=sys.stderr)
    return timelines


def normalize_path(file_path: str, timelines: dict[str, FileTimeline]) -> Optional[str]:
    """Try to match a partial file path to a full path in timelines."""
    if file_path in timelines:
        return file_path
    # Try matching by suffix
    matches = [k for k in timelines if k.endswith(file_path) or k.endswith("/" + file_path)]
    if len(matches) == 1:
        return matches[0]
    if len(matches) > 1:
        print(f"Ambiguous path '{file_path}', matches:", file=sys.stderr)
        for m in matches:
            print(f"  {m}", file=sys.stderr)
        return None
    # Try matching by basename
    basename = os.path.basename(file_path)
    matches = [k for k in timelines if os.path.basename(k) == basename]
    if len(matches) == 1:
        return matches[0]
    if len(matches) > 1:
        print(f"Ambiguous basename '{basename}', matches:", file=sys.stderr)
        for m in matches:
            print(f"  {m}", file=sys.stderr)
    return None


def parse_timestamp_or_index(val: str, timeline: FileTimeline) -> Optional[int]:
    """Parse a timestamp string or numeric index, return the target op index."""
    # Try as integer index first
    try:
        idx = int(val)
        if 0 <= idx < len(timeline.ops):
            return idx
        print(f"Index {idx} out of range (0..{len(timeline.ops)-1})", file=sys.stderr)
        return None
    except ValueError:
        pass

    # Try as timestamp — find the last op at or before this time
    try:
        ts = val.rstrip("Z")
        if "T" not in ts:
            ts = ts + "T23:59:59"
        target = datetime.fromisoformat(ts)
        best_idx = None
        for i, op in enumerate(timeline.ops):
            if op.dt <= target:
                best_idx = i
        if best_idx is not None:
            return best_idx
        print(f"No operations found at or before {val}", file=sys.stderr)
        return None
    except ValueError:
        print(f"Cannot parse '{val}' as index or timestamp", file=sys.stderr)
        return None


# --- Commands ---

def cmd_list(args, project_dir):
    print("Scanning conversation logs...", file=sys.stderr)
    timelines = collect_all_timelines(project_dir)

    # Group by directory for readability
    cwd = str(Path.cwd())
    entries = []
    for path, tl in sorted(timelines.items()):
        rel = path.replace(cwd + "/", "") if path.startswith(cwd) else path
        n_ops = len(tl.ops)
        first_ts = tl.ops[0].timestamp[:19] if tl.ops else "?"
        last_ts = tl.ops[-1].timestamp[:19] if tl.ops else "?"
        op_types = set(o.op_type for o in tl.ops)
        entries.append((rel, n_ops, first_ts, last_ts, op_types))

    print(f"\nFiles touched across all sessions ({len(entries)} files):\n")
    print(f"{'File':<70} {'Ops':>4}  {'First':>19}  {'Last':>19}  Types")
    print("-" * 140)
    for rel, n, first, last, types in entries:
        type_str = ",".join(sorted(t.split("_")[0] for t in types))
        display = rel if len(rel) <= 69 else "..." + rel[-(69-3):]
        print(f"{display:<70} {n:>4}  {first:>19}  {last:>19}  {type_str}")


def cmd_timeline(args, project_dir):
    print("Scanning conversation logs...", file=sys.stderr)
    timelines = collect_all_timelines(project_dir)

    path = normalize_path(args.file_path, timelines)
    if path is None:
        print(f"File not found in history: {args.file_path}", file=sys.stderr)
        sys.exit(1)

    tl = timelines[path]
    cwd = str(Path.cwd())
    rel = path.replace(cwd + "/", "") if path.startswith(cwd) else path

    print(f"\nTimeline for: {rel}")
    print(f"{'Idx':>4}  {'Timestamp':>24}  {'Op':>12}  {'Session':>10}  Details")
    print("-" * 100)
    for i, op in enumerate(tl.ops):
        details = ""
        if op.op_type == "write":
            details = f"content={len(op.content or '')} chars"
        elif op.op_type == "edit":
            details = f"old={len(op.old_string or '')} new={len(op.new_string or '')} chars"
        elif op.op_type.startswith("snapshot"):
            details = f"backup={op.backup_file}"
        print(f"{i:>4}  {op.timestamp[:24]:>24}  {op.op_type:>12}  {op.session_id[:10]:>10}  {details}")


def cmd_snapshot(args, project_dir):
    print("Scanning conversation logs...", file=sys.stderr)
    timelines = collect_all_timelines(project_dir)

    path = normalize_path(args.file_path, timelines)
    if path is None:
        print(f"File not found in history: {args.file_path}", file=sys.stderr)
        sys.exit(1)

    tl = timelines[path]
    target_idx = parse_timestamp_or_index(args.timestamp_or_index, tl)
    if target_idx is None:
        sys.exit(1)

    content = reconstruct_at_index(tl, target_idx)
    if content is None:
        print(f"Cannot reconstruct file at index {target_idx} (no baseline found)", file=sys.stderr)
        sys.exit(1)

    op = tl.ops[target_idx]
    print(f"# Reconstructed: {path}", file=sys.stderr)
    print(f"# At: {op.timestamp} (index {target_idx}, {op.op_type})", file=sys.stderr)

    if args.output:
        Path(args.output).write_text(content)
        print(f"# Written to: {args.output}", file=sys.stderr)
    else:
        sys.stdout.write(content)


def cmd_diff(args, project_dir):
    print("Scanning conversation logs...", file=sys.stderr)
    timelines = collect_all_timelines(project_dir)

    path = normalize_path(args.file_path, timelines)
    if path is None:
        print(f"File not found in history: {args.file_path}", file=sys.stderr)
        sys.exit(1)

    tl = timelines[path]
    target_idx = parse_timestamp_or_index(args.timestamp_or_index, tl)
    if target_idx is None:
        sys.exit(1)

    snapshot_content = reconstruct_at_index(tl, target_idx)
    if snapshot_content is None:
        print(f"Cannot reconstruct file at index {target_idx}", file=sys.stderr)
        sys.exit(1)

    # Read current file from disk
    if os.path.exists(path):
        current_content = Path(path).read_text(errors="replace")
    else:
        current_content = ""
        print(f"# File does not exist on disk: {path}", file=sys.stderr)

    op = tl.ops[target_idx]
    snapshot_lines = snapshot_content.splitlines(keepends=True)
    current_lines = current_content.splitlines(keepends=True)

    diff = difflib.unified_diff(
        snapshot_lines,
        current_lines,
        fromfile=f"{path} (snapshot @ {op.timestamp[:19]})",
        tofile=f"{path} (current on disk)",
        lineterm="",
    )
    diff_text = "\n".join(diff)
    if diff_text:
        print(diff_text)
    else:
        print(f"# No differences — file on disk matches snapshot at index {target_idx}")


# --- Entry point ---

def main():
    parser = argparse.ArgumentParser(
        description="Recover file snapshots from Claude Code conversation logs.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__,
    )
    sub = parser.add_subparsers(dest="command")

    sub.add_parser("list", help="List all files touched across sessions")

    p_tl = sub.add_parser("timeline", help="Show edit timeline for a file")
    p_tl.add_argument("file_path", help="File path (absolute or relative, partial match supported)")

    p_snap = sub.add_parser("snapshot", help="Reconstruct file at a timestamp or index")
    p_snap.add_argument("file_path")
    p_snap.add_argument("timestamp_or_index", help="ISO timestamp or 0-based index from timeline")
    p_snap.add_argument("-o", "--output", help="Write to file instead of stdout")

    p_diff = sub.add_parser("diff", help="Diff snapshot against current file on disk")
    p_diff.add_argument("file_path")
    p_diff.add_argument("timestamp_or_index", help="ISO timestamp or 0-based index from timeline")

    args = parser.parse_args()
    if not args.command:
        parser.print_help()
        sys.exit(1)

    project_dir = detect_project_dir()
    print(f"Project dir: {project_dir}", file=sys.stderr)

    if args.command == "list":
        cmd_list(args, project_dir)
    elif args.command == "timeline":
        cmd_timeline(args, project_dir)
    elif args.command == "snapshot":
        cmd_snapshot(args, project_dir)
    elif args.command == "diff":
        cmd_diff(args, project_dir)


if __name__ == "__main__":
    main()
