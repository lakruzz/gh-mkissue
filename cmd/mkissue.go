// Package cmd provides command-line interface commands for the gh-mkissue extension.
package cmd

func init() {
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
	rootCmd.AddCommand(newTemplateCmd())
	rootCmd.AddCommand(newPullCmd())
	rootCmd.AddCommand(newPushCmd())
	rootCmd.AddCommand(newCreateCmd())
}
