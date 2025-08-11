package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "vscode-finder",
	Short: "A CLI tool to search and open files in VSCode",
	Long:  `vscode-finder lets you search for files by name or content, and open them directly in VSCode.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// Subcommands will be added here
}
