// Package cmd provides command-line interface commands for the gh-utils extension.
package cmd

import "github.com/spf13/cobra"

var mkissueCmd = &cobra.Command{
	Use:   "mkissue",
	Short: "Create a GitHub issue from a markdown file",
	Long: `Create a GitHub issue from a markdown file with frontmatter support.
The markdown file should contain YAML frontmatter with metadata and a markdown body.

Usage variants:
	utils mkissue create --file <file> [--branch <branch>] [--repo <owner/repo>]
	utils mkissue create --file <file> [--gist <gist-id>]
  utils mkissue template [--elaborate] [--out <file>]
  utils mkissue pull <issue-numbers|open>
	utils mkissue push <issue-numbers>

Rules:
  --file is required when creating an issue
  --branch is optional (defaults to the repo's default branch when used with --repo)
  --gist and --repo are mutually exclusive
  --branch is not valid with --gist`,
}

func init() {
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
	rootCmd.AddCommand(mkissueCmd)
	mkissueCmd.AddCommand(newTemplateCmd())
	mkissueCmd.AddCommand(newPullCmd())
	mkissueCmd.AddCommand(newPushCmd())
	mkissueCmd.AddCommand(newCreateCmd())
}
