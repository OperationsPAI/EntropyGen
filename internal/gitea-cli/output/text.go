package output

import (
	"fmt"
	"os"
	"strings"
)

// FormatIssueList formats an issue list entry.
// Format: #42  [priority/high,type/bug]  Fix memory leak  (john)
func FormatIssueList(number int64, labels []string, title string, assignees []string) string {
	labelStr := strings.Join(labels, ",")

	assigneeStr := "unassigned"
	if len(assignees) > 0 {
		assigneeStr = strings.Join(assignees, ",")
	}

	return fmt.Sprintf("#%d  [%s]  %s  (%s)", number, labelStr, title, assigneeStr)
}

// FormatPRList formats a PR list entry.
// Format: #10  open  Add feature X  (feature-branch)
func FormatPRList(number int64, state, title, head string) string {
	return fmt.Sprintf("#%d  %s  %s  (%s)", number, state, title, head)
}

// FormatNotification formats a notification entry.
// Format: [unread] #42  Subject title  repo-name
func FormatNotification(id int64, subject, repoName string, unread bool) string {
	status := "read"
	if unread {
		status = "unread"
	}
	return fmt.Sprintf("[%s] #%d  %s  %s", status, id, subject, repoName)
}

// PrintError prints an error message to stderr.
func PrintError(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
}
