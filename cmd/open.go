package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	openDir bool
)

var openCmd = &cobra.Command{
	Use:   "open [file]",
	Short: "Open file or directory in VS Code",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := args[0]

		// Check if path exists
		if _, err := os.Stat(path); os.IsNotExist(err) {
			fmt.Printf("Error: '%s' does not exist\n", path)
			return
		}

		// If --dir flag is set, get the containing directory
		if openDir {
			fileInfo, err := os.Stat(path)
			if err != nil {
				fmt.Printf("Error: Unable to get file info: %v\n", err)
				return
			}
			if !fileInfo.IsDir() {
				path = filepath.Dir(path)
			}
		}

		// Get absolute path
		absPath, err := filepath.Abs(path)
		if err != nil {
			fmt.Printf("Error: Unable to get absolute path: %v\n", err)
			return
		}

		// Open in VS Code using 'code' command
		vscodeCmd := exec.Command("code", absPath)
		if err := vscodeCmd.Run(); err != nil {
			fmt.Printf("Error: Failed to open VS Code: %v\n", err)
			fmt.Println("Make sure VS Code is installed and 'code' command is available in PATH")
			return
		}

		fmt.Printf("Opened in VS Code: %s\n", absPath)
	},
}

func init() {
	rootCmd.AddCommand(openCmd)
	openCmd.Flags().BoolVarP(&openDir, "dir", "d", false, "Open the containing directory instead of the file")
}
