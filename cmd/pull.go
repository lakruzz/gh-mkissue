package cmd

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/lakruzz/gh-utils/cmd/mkissue"
	"github.com/spf13/cobra"
)

func parseIssueNumbers(selection string) ([]int, error) {
	var numbers []int
	seen := make(map[int]bool)
	for _, value := range strings.Split(selection, ",") {
		if value == "" {
			return nil, fmt.Errorf("issue numbers must be comma-separated positive integers without spaces")
		}
		for _, character := range value {
			if character < '0' || character > '9' {
				return nil, fmt.Errorf("issue numbers must be comma-separated positive integers without spaces")
			}
		}
		number, err := strconv.Atoi(value)
		if err != nil || number <= 0 {
			return nil, fmt.Errorf("issue numbers must be comma-separated positive integers without spaces")
		}
		if !seen[number] {
			seen[number] = true
			numbers = append(numbers, number)
		}
	}
	return numbers, nil
}

func newPullCmd() *cobra.Command {
	return newPullCmdWith(mkissue.ListOpenIssueNumbers, mkissue.PullTo)
}

func newPullCmdWith(listOpen func() ([]int, error), pull func(int, string, bool) (string, error)) *cobra.Command {
	var directory string
	var force bool
	command := &cobra.Command{
		Use:   "pull <issue-numbers|open>",
		Short: "Pull issues into local Markdown files",
		Long:  "Pull a comma-separated list of issue numbers, or all open issues, into local Markdown files.",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if command.Flags().Changed("dir") && directory == "" {
				return fmt.Errorf("directory cannot be empty")
			}
			var numbers []int
			var err error
			if args[0] == "open" {
				numbers, err = listOpen()
			} else {
				numbers, err = parseIssueNumbers(args[0])
			}
			if err != nil {
				return err
			}
			if len(numbers) == 0 {
				_, err := fmt.Fprintln(command.OutOrStdout(), "No open issues found.")
				return err
			}
			return pullIssues(command.OutOrStdout(), numbers, directory, force, pull)
		},
	}
	command.Flags().StringVarP(&directory, "dir", "d", "", "Output directory (default: issues at repository root)")
	command.Flags().BoolVarP(&force, "force", "f", false, "Replace an existing draft for this issue")
	return command
}

func pullIssues(output io.Writer, numbers []int, directory string, force bool, pull func(int, string, bool) (string, error)) error {
	var failures []error
	for _, number := range numbers {
		path, err := pull(number, directory, force)
		if err != nil {
			failures = append(failures, fmt.Errorf("issue #%d: %w", number, err))
			continue
		}
		if _, err := fmt.Fprintf(output, "Created %s\n", path); err != nil {
			return fmt.Errorf("report pulled issue #%d: %w", number, err)
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("failed to pull %d of %d issues: %w", len(failures), len(numbers), errors.Join(failures...))
	}
	return nil
}
