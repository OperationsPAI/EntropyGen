package commands

import (
	"fmt"
	"time"

	sdk "code.gitea.io/sdk/gitea"
	"github.com/entropyGen/entropyGen/internal/gitea-cli/output"
	"github.com/spf13/cobra"
)

// NewNotifyCmd returns the "notify" parent command with all subcommands attached.
func NewNotifyCmd(jsonMode *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "notify",
		Short: "Manage notifications",
	}

	cmd.AddCommand(
		newNotifyListCmd(jsonMode),
		newNotifyReadCmd(jsonMode),
		newNotifyReadAllCmd(jsonMode),
	)

	return cmd
}

func newNotifyListCmd(jsonMode *bool) *cobra.Command {
	var (
		unread bool
		since  string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List notifications",
		RunE: func(_ *cobra.Command, _ []string) error {
			client, err := newClient()
			if err != nil {
				return err
			}

			opts := sdk.ListNotificationOptions{
				ListOptions: sdk.ListOptions{Page: 1, PageSize: 50},
			}

			if unread {
				opts.Status = []sdk.NotifyStatus{sdk.NotifyStatusUnread}
			}

			if since != "" {
				t, err := time.Parse(time.RFC3339, since)
				if err != nil {
					return fmt.Errorf("parse --since as RFC3339: %w", err)
				}
				opts.Since = t
			}

			notifications, _, err := client.ListNotifications(opts)
			if err != nil {
				return fmt.Errorf("list notifications: %w", err)
			}

			if *jsonMode {
				return output.PrintJSON(notifications)
			}

			for _, n := range notifications {
				subject := ""
				if n.Subject != nil {
					subject = n.Subject.Title
				}
				repoName := ""
				if n.Repository != nil {
					repoName = n.Repository.FullName
				}
				fmt.Println(output.FormatNotification(n.ID, subject, repoName, n.Unread))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&unread, "unread", true, "Show only unread notifications (use --unread=false for all)")
	cmd.Flags().StringVar(&since, "since", "", "Show notifications since (RFC3339 format)")

	return cmd
}

func newNotifyReadCmd(jsonMode *bool) *cobra.Command {
	var threadID int64

	cmd := &cobra.Command{
		Use:   "read",
		Short: "Mark a notification thread as read",
		RunE: func(_ *cobra.Command, _ []string) error {
			client, err := newClient()
			if err != nil {
				return err
			}

			thread, _, err := client.ReadNotification(threadID)
			if err != nil {
				return fmt.Errorf("read notification %d: %w", threadID, err)
			}

			if *jsonMode {
				return output.PrintJSON(thread)
			}

			fmt.Printf("Marked notification #%d as read\n", threadID)
			return nil
		},
	}

	cmd.Flags().Int64Var(&threadID, "thread-id", 0, "Notification thread ID (required)")
	_ = cmd.MarkFlagRequired("thread-id")

	return cmd
}

func newNotifyReadAllCmd(jsonMode *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "read-all",
		Short: "Mark all notifications as read",
		RunE: func(_ *cobra.Command, _ []string) error {
			client, err := newClient()
			if err != nil {
				return err
			}

			threads, _, err := client.ReadNotifications(sdk.MarkNotificationOptions{})
			if err != nil {
				return fmt.Errorf("read all notifications: %w", err)
			}

			if *jsonMode {
				return output.PrintJSON(threads)
			}

			fmt.Println("Marked all notifications as read")
			return nil
		},
	}

	return cmd
}
