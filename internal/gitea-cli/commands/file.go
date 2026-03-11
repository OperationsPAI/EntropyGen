package commands

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/entropyGen/entropyGen/internal/gitea-cli/output"
	"github.com/spf13/cobra"
)

// NewFileCmd returns the "file" parent command with all subcommands attached.
func NewFileCmd(jsonMode *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "file",
		Short: "Manage repository files",
	}

	cmd.AddCommand(
		newFileGetCmd(jsonMode),
	)

	return cmd
}

func newFileGetCmd(jsonMode *bool) *cobra.Command {
	var (
		repo     string
		filePath string
		ref      string
	)

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get file content from a repository",
		RunE: func(_ *cobra.Command, _ []string) error {
			owner, repoName, err := splitRepo(repo)
			if err != nil {
				return err
			}

			client, err := newClient()
			if err != nil {
				return err
			}

			contents, _, err := client.GetContents(owner, repoName, ref, filePath)
			if err != nil {
				return fmt.Errorf("get file %s: %w", filePath, err)
			}

			if *jsonMode {
				return output.PrintJSON(contents)
			}

			// Content is base64-encoded; decode and print raw bytes to stdout.
			if contents.Content == nil {
				return fmt.Errorf("file %s has no content (may be a directory or symlink)", filePath)
			}

			decoded, err := base64.StdEncoding.DecodeString(*contents.Content)
			if err != nil {
				return fmt.Errorf("decode base64 content: %w", err)
			}

			_, err = os.Stdout.Write(decoded)
			return err
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "Repository in org/repo format (required)")
	cmd.Flags().StringVar(&filePath, "path", "", "File path in repository (required)")
	cmd.Flags().StringVar(&ref, "ref", "main", "Git ref (branch, tag, or commit)")
	_ = cmd.MarkFlagRequired("repo")
	_ = cmd.MarkFlagRequired("path")

	return cmd
}
