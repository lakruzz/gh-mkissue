package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/lakruzz/gh-mkissue/templates"
	"github.com/spf13/cobra"
)

func newTemplateCmd() *cobra.Command {
	var elaborate bool
	var output string

	command := &cobra.Command{
		Use:   "template",
		Short: "Create an issue template",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			template := templates.MinimalTemplate
			if elaborate {
				template = templates.ElaborateTemplate
			}
			if output == "-" {
				_, err := io.WriteString(command.OutOrStdout(), template)
				return err
			}
			// #nosec G304 -- output path is supplied by the user
			file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if err != nil {
				return fmt.Errorf("create template file: %w", err)
			}
			_, writeErr := io.WriteString(file, template)
			closeErr := file.Close()
			if writeErr != nil {
				return fmt.Errorf("write template file: %w", writeErr)
			}
			if closeErr != nil {
				return fmt.Errorf("close template file: %w", closeErr)
			}
			_, err = fmt.Fprintf(command.OutOrStdout(), "Created %s\n", output)
			return err
		},
	}
	command.Flags().BoolVarP(&elaborate, "elaborate", "e", false, "Include the full issue template reference")
	command.Flags().StringVarP(&output, "out", "o", "template.issue.md", "Output file (use - for stdout)")
	return command
}
