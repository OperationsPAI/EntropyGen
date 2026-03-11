package commands

import (
	"fmt"
	"os"
	"strings"

	sdk "code.gitea.io/sdk/gitea"
	"github.com/entropyGen/entropyGen/internal/gitea-cli/output"
	"github.com/spf13/cobra"
)

// NewIssueCmd returns the "issue" parent command with all subcommands attached.
func NewIssueCmd(jsonMode *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "issue",
		Short: "Manage issues",
	}

	cmd.AddCommand(
		newIssueListCmd(jsonMode),
		newIssueCreateCmd(jsonMode),
		newIssueAssignCmd(jsonMode),
		newIssueCommentCmd(jsonMode),
		newIssueCloseCmd(jsonMode),
	)

	return cmd
}

func newIssueListCmd(jsonMode *bool) *cobra.Command {
	var (
		repo     string
		label    string
		state    string
		assignee string
		limit    int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List issues in a repository",
		RunE: func(_ *cobra.Command, _ []string) error {
			owner, repoName, err := splitRepo(repo)
			if err != nil {
				return err
			}

			client, err := newClient()
			if err != nil {
				return err
			}

			var labels []string
			if label != "" {
				labels = strings.Split(label, ",")
			}

			var stateType sdk.StateType
			switch state {
			case "closed":
				stateType = sdk.StateClosed
			case "all":
				stateType = sdk.StateAll
			default:
				stateType = sdk.StateOpen
			}

			opts := sdk.ListIssueOption{
				ListOptions: sdk.ListOptions{
					Page:     1,
					PageSize: limit,
				},
				State:      stateType,
				Labels:     labels,
				AssignedBy: assignee,
			}

			issues, _, err := client.ListRepoIssues(owner, repoName, opts)
			if err != nil {
				return fmt.Errorf("list issues: %w", err)
			}

			if *jsonMode {
				return output.PrintJSON(issues)
			}

			for _, issue := range issues {
				var labelNames []string
				for _, l := range issue.Labels {
					labelNames = append(labelNames, l.Name)
				}
				var assigneeNames []string
				for _, a := range issue.Assignees {
					assigneeNames = append(assigneeNames, a.UserName)
				}
				fmt.Println(output.FormatIssueList(issue.Index, labelNames, issue.Title, assigneeNames))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "Repository in org/repo format (required)")
	cmd.Flags().StringVar(&label, "label", "", "Comma-separated label filter")
	cmd.Flags().StringVar(&state, "state", "open", "Issue state: open, closed, or all")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Filter by assignee username")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of issues to list")
	_ = cmd.MarkFlagRequired("repo")

	return cmd
}

func newIssueCreateCmd(jsonMode *bool) *cobra.Command {
	var (
		repo      string
		title     string
		body      string
		labels    string
		assignees string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new issue",
		RunE: func(_ *cobra.Command, _ []string) error {
			owner, repoName, err := splitRepo(repo)
			if err != nil {
				return err
			}

			client, err := newClient()
			if err != nil {
				return err
			}

			opt := sdk.CreateIssueOption{
				Title: title,
				Body:  body,
			}

			if assignees != "" {
				opt.Assignees = strings.Split(assignees, ",")
			}

			// Labels in CreateIssueOption are IDs (int64), but the user provides names.
			// We need to resolve label names to IDs.
			if labels != "" {
				labelNames := strings.Split(labels, ",")
				labelIDs, err := resolveLabelIDs(client, owner, repoName, labelNames)
				if err != nil {
					return err
				}
				opt.Labels = labelIDs
			}

			issue, _, err := client.CreateIssue(owner, repoName, opt)
			if err != nil {
				return fmt.Errorf("create issue: %w", err)
			}

			if *jsonMode {
				return output.PrintJSON(issue)
			}

			fmt.Fprintf(os.Stdout, "Created issue #%d: %s\n", issue.Index, issue.Title)
			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "Repository in org/repo format (required)")
	cmd.Flags().StringVar(&title, "title", "", "Issue title (required)")
	cmd.Flags().StringVar(&body, "body", "", "Issue body (required)")
	cmd.Flags().StringVar(&labels, "labels", "", "Comma-separated label names")
	cmd.Flags().StringVar(&assignees, "assignees", "", "Comma-separated assignee usernames")
	_ = cmd.MarkFlagRequired("repo")
	_ = cmd.MarkFlagRequired("title")
	_ = cmd.MarkFlagRequired("body")

	return cmd
}

func newIssueAssignCmd(jsonMode *bool) *cobra.Command {
	var (
		repo     string
		number   int64
		assignee string
	)

	cmd := &cobra.Command{
		Use:   "assign",
		Short: "Assign an issue to a user",
		RunE: func(_ *cobra.Command, _ []string) error {
			owner, repoName, err := splitRepo(repo)
			if err != nil {
				return err
			}

			client, err := newClient()
			if err != nil {
				return err
			}

			// If no assignee specified, use the token owner
			if assignee == "" {
				user, _, err := client.GetMyUserInfo()
				if err != nil {
					return fmt.Errorf("get current user: %w", err)
				}
				assignee = user.UserName
			}

			issue, _, err := client.EditIssue(owner, repoName, number, sdk.EditIssueOption{
				Assignees: []string{assignee},
			})
			if err != nil {
				return fmt.Errorf("assign issue: %w", err)
			}

			if *jsonMode {
				return output.PrintJSON(issue)
			}

			fmt.Fprintf(os.Stdout, "Assigned issue #%d to %s\n", number, assignee)
			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "Repository in org/repo format (required)")
	cmd.Flags().Int64Var(&number, "number", 0, "Issue number (required)")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Assignee username (default: token owner)")
	_ = cmd.MarkFlagRequired("repo")
	_ = cmd.MarkFlagRequired("number")

	return cmd
}

func newIssueCommentCmd(jsonMode *bool) *cobra.Command {
	var (
		repo   string
		number int64
		body   string
	)

	cmd := &cobra.Command{
		Use:   "comment",
		Short: "Add a comment to an issue",
		RunE: func(_ *cobra.Command, _ []string) error {
			owner, repoName, err := splitRepo(repo)
			if err != nil {
				return err
			}

			client, err := newClient()
			if err != nil {
				return err
			}

			comment, _, err := client.CreateIssueComment(owner, repoName, number, sdk.CreateIssueCommentOption{
				Body: body,
			})
			if err != nil {
				return fmt.Errorf("create comment: %w", err)
			}

			if *jsonMode {
				return output.PrintJSON(comment)
			}

			fmt.Fprintf(os.Stdout, "Added comment to issue #%d\n", number)
			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "Repository in org/repo format (required)")
	cmd.Flags().Int64Var(&number, "number", 0, "Issue number (required)")
	cmd.Flags().StringVar(&body, "body", "", "Comment body (required)")
	_ = cmd.MarkFlagRequired("repo")
	_ = cmd.MarkFlagRequired("number")
	_ = cmd.MarkFlagRequired("body")

	return cmd
}

func newIssueCloseCmd(jsonMode *bool) *cobra.Command {
	var (
		repo   string
		number int64
	)

	cmd := &cobra.Command{
		Use:   "close",
		Short: "Close an issue",
		RunE: func(_ *cobra.Command, _ []string) error {
			owner, repoName, err := splitRepo(repo)
			if err != nil {
				return err
			}

			client, err := newClient()
			if err != nil {
				return err
			}

			closedState := sdk.StateClosed
			issue, _, err := client.EditIssue(owner, repoName, number, sdk.EditIssueOption{
				State: &closedState,
			})
			if err != nil {
				return fmt.Errorf("close issue: %w", err)
			}

			if *jsonMode {
				return output.PrintJSON(issue)
			}

			fmt.Fprintf(os.Stdout, "Closed issue #%d\n", number)
			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "Repository in org/repo format (required)")
	cmd.Flags().Int64Var(&number, "number", 0, "Issue number (required)")
	_ = cmd.MarkFlagRequired("repo")
	_ = cmd.MarkFlagRequired("number")

	return cmd
}

// resolveLabelIDs looks up label IDs by name for a given repository.
// It paginates through all labels to handle repos with more than 50 labels.
func resolveLabelIDs(client *sdk.Client, owner, repo string, names []string) ([]int64, error) {
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[strings.TrimSpace(n)] = true
	}

	const pageSize = 50
	var allLabels []*sdk.Label
	page := 1
	for {
		labels, _, err := client.ListRepoLabels(owner, repo, sdk.ListLabelsOptions{
			ListOptions: sdk.ListOptions{Page: page, PageSize: pageSize},
		})
		if err != nil {
			return nil, fmt.Errorf("list repo labels: %w", err)
		}
		allLabels = append(allLabels, labels...)
		if len(labels) < pageSize {
			break
		}
		page++
	}

	ids := make([]int64, 0, len(names))
	for _, l := range allLabels {
		if nameSet[l.Name] {
			ids = append(ids, l.ID)
		}
	}

	// Verify all requested labels were found.
	found := make(map[string]bool, len(ids))
	for _, l := range allLabels {
		if nameSet[l.Name] {
			found[l.Name] = true
		}
	}
	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if !found[trimmed] {
			return nil, fmt.Errorf("label %q not found in repository %s/%s", trimmed, owner, repo)
		}
	}

	return ids, nil
}
