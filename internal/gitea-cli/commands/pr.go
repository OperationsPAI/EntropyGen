package commands

import (
	"fmt"
	"os"
	"strings"

	sdk "code.gitea.io/sdk/gitea"
	"github.com/entropyGen/entropyGen/internal/gitea-cli/output"
	"github.com/spf13/cobra"
)

// NewPRCmd returns the "pr" parent command with all subcommands attached.
func NewPRCmd(jsonMode *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pr",
		Short: "Manage pull requests",
	}

	cmd.AddCommand(
		newPRListCmd(jsonMode),
		newPRCreateCmd(jsonMode),
		newPRReviewCmd(jsonMode),
		newPRReviewsCmd(jsonMode),
		newPRMergeCmd(jsonMode),
	)

	return cmd
}

func newPRListCmd(jsonMode *bool) *cobra.Command {
	var (
		repo  string
		state string
		label string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List pull requests in a repository",
		RunE: func(_ *cobra.Command, _ []string) error {
			owner, repoName, err := splitRepo(repo)
			if err != nil {
				return err
			}

			client, err := newClient()
			if err != nil {
				return err
			}

			var stateType sdk.StateType
			filterMerged := false
			switch state {
			case "closed":
				stateType = sdk.StateClosed
			case "merged":
				// Gitea treats merged PRs as closed; filter client-side.
				stateType = sdk.StateClosed
				filterMerged = true
			case "all":
				stateType = sdk.StateAll
			default:
				stateType = sdk.StateOpen
			}

			prs, _, err := client.ListRepoPullRequests(owner, repoName, sdk.ListPullRequestsOptions{
				ListOptions: sdk.ListOptions{Page: 1, PageSize: 20},
				State:       stateType,
			})
			if err != nil {
				return fmt.Errorf("list pull requests: %w", err)
			}

			// Filter for merged PRs when --state=merged was requested.
			if filterMerged {
				prs = filterMergedPRs(prs)
			}

			// Client-side label filtering since the SDK's ListPullRequestsOptions
			// does not support label filtering.
			if label != "" {
				prs = filterPRsByLabels(prs, label)
			}

			if *jsonMode {
				return output.PrintJSON(prs)
			}

			for _, pr := range prs {
				headRef := ""
				if pr.Head != nil {
					headRef = pr.Head.Ref
				}
				fmt.Println(output.FormatPRList(pr.Index, string(pr.State), pr.Title, headRef))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "Repository in org/repo format (required)")
	cmd.Flags().StringVar(&state, "state", "open", "PR state: open, closed, merged, or all")
	cmd.Flags().StringVar(&label, "label", "", "Comma-separated label filter")
	_ = cmd.MarkFlagRequired("repo")

	return cmd
}

func newPRCreateCmd(jsonMode *bool) *cobra.Command {
	var (
		repo  string
		title string
		body  string
		head  string
		base  string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new pull request",
		RunE: func(_ *cobra.Command, _ []string) error {
			owner, repoName, err := splitRepo(repo)
			if err != nil {
				return err
			}

			client, err := newClient()
			if err != nil {
				return err
			}

			pr, _, err := client.CreatePullRequest(owner, repoName, sdk.CreatePullRequestOption{
				Head:  head,
				Base:  base,
				Title: title,
				Body:  body,
			})
			if err != nil {
				return fmt.Errorf("create pull request: %w", err)
			}

			if *jsonMode {
				return output.PrintJSON(pr)
			}

			fmt.Fprintf(os.Stdout, "Created PR #%d: %s\n", pr.Index, pr.Title)
			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "Repository in org/repo format (required)")
	cmd.Flags().StringVar(&title, "title", "", "PR title (required)")
	cmd.Flags().StringVar(&body, "body", "", "PR body (required)")
	cmd.Flags().StringVar(&head, "head", "", "Head branch (required)")
	cmd.Flags().StringVar(&base, "base", "main", "Base branch")
	_ = cmd.MarkFlagRequired("repo")
	_ = cmd.MarkFlagRequired("title")
	_ = cmd.MarkFlagRequired("body")
	_ = cmd.MarkFlagRequired("head")

	return cmd
}

func newPRReviewCmd(jsonMode *bool) *cobra.Command {
	var (
		repo   string
		number int64
		event  string
		body   string
	)

	cmd := &cobra.Command{
		Use:   "review",
		Short: "Submit a pull request review",
		RunE: func(_ *cobra.Command, _ []string) error {
			owner, repoName, err := splitRepo(repo)
			if err != nil {
				return err
			}

			client, err := newClient()
			if err != nil {
				return err
			}

			reviewState, err := parseReviewEvent(event)
			if err != nil {
				return err
			}

			review, _, err := client.CreatePullReview(owner, repoName, number, sdk.CreatePullReviewOptions{
				State: reviewState,
				Body:  body,
			})
			if err != nil {
				return fmt.Errorf("create review: %w", err)
			}

			if *jsonMode {
				return output.PrintJSON(review)
			}

			fmt.Fprintf(os.Stdout, "Submitted %s review on PR #%d\n", event, number)
			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "Repository in org/repo format (required)")
	cmd.Flags().Int64Var(&number, "number", 0, "PR number (required)")
	cmd.Flags().StringVar(&event, "event", "", "Review event: APPROVE, REQUEST_CHANGES, or COMMENT (required)")
	cmd.Flags().StringVar(&body, "body", "", "Review body (required)")
	_ = cmd.MarkFlagRequired("repo")
	_ = cmd.MarkFlagRequired("number")
	_ = cmd.MarkFlagRequired("event")
	_ = cmd.MarkFlagRequired("body")

	return cmd
}

func newPRMergeCmd(jsonMode *bool) *cobra.Command {
	var (
		repo   string
		number int64
		method string
	)

	cmd := &cobra.Command{
		Use:   "merge",
		Short: "Merge a pull request",
		RunE: func(_ *cobra.Command, _ []string) error {
			owner, repoName, err := splitRepo(repo)
			if err != nil {
				return err
			}

			client, err := newClient()
			if err != nil {
				return err
			}

			mergeStyle, err := parseMergeMethod(method)
			if err != nil {
				return err
			}

			_, _, err = client.MergePullRequest(owner, repoName, number, sdk.MergePullRequestOption{
				Style: mergeStyle,
			})
			if err != nil {
				return fmt.Errorf("merge pull request: %w", err)
			}

			if *jsonMode {
				return output.PrintJSON(map[string]any{
					"number": number,
					"merged": true,
					"method": method,
				})
			}

			fmt.Fprintf(os.Stdout, "Merged PR #%d via %s\n", number, method)
			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "Repository in org/repo format (required)")
	cmd.Flags().Int64Var(&number, "number", 0, "PR number (required)")
	cmd.Flags().StringVar(&method, "method", "merge", "Merge method: merge, squash, or rebase")
	_ = cmd.MarkFlagRequired("repo")
	_ = cmd.MarkFlagRequired("number")

	return cmd
}

func newPRReviewsCmd(jsonMode *bool) *cobra.Command {
	var (
		repo   string
		number int64
	)

	cmd := &cobra.Command{
		Use:   "reviews",
		Short: "List reviews for a pull request",
		RunE: func(_ *cobra.Command, _ []string) error {
			owner, repoName, err := splitRepo(repo)
			if err != nil {
				return err
			}

			client, err := newClient()
			if err != nil {
				return err
			}

			reviews, _, err := client.ListPullReviews(owner, repoName, number, sdk.ListPullReviewsOptions{})
			if err != nil {
				return fmt.Errorf("list reviews: %w", err)
			}

			if *jsonMode {
				return output.PrintJSON(reviews)
			}

			approved := 0
			changesRequested := 0
			for _, r := range reviews {
				switch r.State {
				case sdk.ReviewStateApproved:
					approved++
					fmt.Fprintf(os.Stdout, "APPROVED     by %s\n", r.Reviewer.UserName)
				case sdk.ReviewStateRequestChanges:
					changesRequested++
					fmt.Fprintf(os.Stdout, "CHANGES_REQ  by %s\n", r.Reviewer.UserName)
				case sdk.ReviewStateComment:
					fmt.Fprintf(os.Stdout, "COMMENT      by %s\n", r.Reviewer.UserName)
				}
			}
			fmt.Fprintf(os.Stdout, "\napproved=%d changes_requested=%d\n", approved, changesRequested)
			if approved > 0 && changesRequested == 0 {
				fmt.Fprintln(os.Stdout, "status=ready_to_merge")
			} else {
				fmt.Fprintln(os.Stdout, "status=not_ready")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "Repository in org/repo format (required)")
	cmd.Flags().Int64Var(&number, "number", 0, "PR number (required)")
	_ = cmd.MarkFlagRequired("repo")
	_ = cmd.MarkFlagRequired("number")

	return cmd
}

// parseReviewEvent maps a string event to the SDK ReviewStateType.
func parseReviewEvent(event string) (sdk.ReviewStateType, error) {
	switch event {
	case "APPROVE":
		return sdk.ReviewStateApproved, nil
	case "REQUEST_CHANGES":
		return sdk.ReviewStateRequestChanges, nil
	case "COMMENT":
		return sdk.ReviewStateComment, nil
	default:
		return "", fmt.Errorf("invalid review event %q: must be APPROVE, REQUEST_CHANGES, or COMMENT", event)
	}
}

// parseMergeMethod maps a string method to the SDK MergeStyle.
func parseMergeMethod(method string) (sdk.MergeStyle, error) {
	switch method {
	case "merge":
		return sdk.MergeStyleMerge, nil
	case "squash":
		return sdk.MergeStyleSquash, nil
	case "rebase":
		return sdk.MergeStyleRebase, nil
	default:
		return "", fmt.Errorf("invalid merge method %q: must be merge, squash, or rebase", method)
	}
}

// filterMergedPRs returns only PRs that have been merged.
func filterMergedPRs(prs []*sdk.PullRequest) []*sdk.PullRequest {
	var merged []*sdk.PullRequest
	for _, pr := range prs {
		if pr.HasMerged {
			merged = append(merged, pr)
		}
	}
	return merged
}

// filterPRsByLabels filters PRs that have at least one of the specified labels.
func filterPRsByLabels(prs []*sdk.PullRequest, labelFilter string) []*sdk.PullRequest {
	wanted := make(map[string]bool)
	for _, l := range strings.Split(labelFilter, ",") {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			wanted[trimmed] = true
		}
	}

	var filtered []*sdk.PullRequest
	for _, pr := range prs {
		for _, l := range pr.Labels {
			if wanted[l.Name] {
				filtered = append(filtered, pr)
				break
			}
		}
	}
	return filtered
}
