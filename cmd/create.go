package cmd

import (
	"fmt"

	"github.com/lakruzz/gh-utils/cmd/mkissue"
	"github.com/spf13/cobra"
)

func newCreateCmd() *cobra.Command {
	return newCreateCmdWith(mkissue.Create)
}

func newCreateCmdWith(create func(mkissue.CreateOptions) error) *cobra.Command {
	var issueFile, branch, gist, repo string
	var clean, createLabels bool
	command := &cobra.Command{
		Use:   "create",
		Short: "Create a GitHub issue from a Markdown file",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if branch != "" && gist != "" {
				return fmt.Errorf("cannot use both --branch and --gist flags together")
			}
			if repo != "" && gist != "" {
				return fmt.Errorf("cannot use both --repo and --gist flags together")
			}
			return create(mkissue.CreateOptions{File: issueFile, Branch: branch, Gist: gist, Repo: repo, Clean: clean, CreateLabels: createLabels})
		},
	}
	command.Flags().StringVarP(&issueFile, "file", "f", "", "Path to the issue Markdown file (required)")
	command.Flags().StringVarP(&branch, "branch", "b", "", "Branch name to get the file from")
	command.Flags().StringVarP(&gist, "gist", "g", "", "Gist ID to get the file from")
	command.Flags().StringVarP(&repo, "repo", "r", "", "Repository to get the file from, in owner/repo format")
	command.Flags().BoolVarP(&clean, "clean", "c", false, "Delete the unchanged local source after successful creation")
	command.Flags().BoolVar(&createLabels, "create-labels", false, "Create missing labels")
	_ = command.MarkFlagRequired("file")
	return command
}
