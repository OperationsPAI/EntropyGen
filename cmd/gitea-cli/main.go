package main

import (
	"os"

	"github.com/entropyGen/entropyGen/internal/gitea-cli/commands"
	"github.com/spf13/cobra"
)

func main() {
	var jsonMode bool

	root := &cobra.Command{
		Use:   "gitea",
		Short: "Gitea CLI for agent operations",
	}
	root.PersistentFlags().BoolVar(&jsonMode, "json", false, "Output as JSON")

	root.AddCommand(
		commands.NewIssueCmd(&jsonMode),
		commands.NewPRCmd(&jsonMode),
		commands.NewNotifyCmd(&jsonMode),
		commands.NewFileCmd(&jsonMode),
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
