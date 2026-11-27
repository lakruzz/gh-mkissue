package cmd

import (
	"fmt"
	"io"

	"github.com/lakruzz/gh-mkissue/cmd/mkissue"
	"github.com/spf13/cobra"
)

func newPushCmd() *cobra.Command {
	return newPushCmdWith(mkissue.PushWithOptions)
}

func newPushCmdWith(push func([]int, mkissue.PushOptions, io.Writer) error) *cobra.Command {
	var directory string
	var force, clean, createLabels bool
	command := &cobra.Command{
		Use:   "push <issue-numbers>",
		Short: "Update issues from local Markdown files",
		Long:  "Update issues from a comma-separated list of issue numbers. Body changes on issues with comments require --force.",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if command.Flags().Changed("dir") && directory == "" {
				return fmt.Errorf("directory cannot be empty")
			}
			numbers, err := parseIssueNumbers(args[0])
			if err != nil {
				return err
			}
			return push(numbers, mkissue.PushOptions{Directory: directory, Force: force, Clean: clean, CreateLabels: createLabels}, command.OutOrStdout())
		},
	}
	command.Flags().StringVarP(&directory, "dir", "d", "", "Draft directory (default: issues at repository root)")
	command.Flags().BoolVarP(&force, "force", "f", false, "Allow description changes on issues with comments")
	command.Flags().BoolVarP(&clean, "clean", "c", false, "Delete unchanged local drafts after successful updates")
	command.Flags().BoolVar(&createLabels, "create-labels", false, "Create missing labels")
	return command
}
