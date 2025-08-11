package cmd

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	searchName    string
	searchContent string
	searchDir     string
)

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search for files by name or content",
	Run: func(cmd *cobra.Command, args []string) {
		// Validate search directory
		if _, err := os.Stat(searchDir); os.IsNotExist(err) {
			fmt.Printf("Error: Directory '%s' does not exist\n", searchDir)
			return
		}

		fmt.Printf("Searching in: %s\n", searchDir)
		matches := make(map[string]bool)

		err := filepath.Walk(searchDir, func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Skip directories
			if info.IsDir() {
				return nil
			}

			// Check filename match if searchName is provided
			if searchName != "" {
				matched, err := filepath.Match(strings.ToLower(searchName), strings.ToLower(filepath.Base(path)))
				if err != nil {
					return err
				}
				if matched {
					matches[path] = true
				}
			}

			// Check content match if searchContent is provided
			if searchContent != "" && !matches[path] {
				file, err := os.Open(path)
				if err != nil {
					return nil // Skip files we can't open
				}
				defer file.Close()

				scanner := bufio.NewScanner(file)
				lineNum := 1
				for scanner.Scan() {
					if strings.Contains(scanner.Text(), searchContent) {
						matches[path] = true
						fmt.Printf("%s:%d: %s\n", path, lineNum, scanner.Text())
					}
					lineNum++
				}
			}

			return nil
		})

		if err != nil {
			fmt.Printf("Error during search: %v\n", err)
			return
		}

		// Print results if no content matches were already printed
		if searchContent == "" {
			for path := range matches {
				fmt.Println(path)
			}
		}

		if len(matches) == 0 {
			fmt.Println("No matches found")
		}
	},
}

func init() {
	rootCmd.AddCommand(searchCmd)

	searchCmd.Flags().StringVarP(&searchName, "name", "n", "", "Search files by name pattern")
	searchCmd.Flags().StringVarP(&searchContent, "content", "c", "", "Search files by content")
	searchCmd.Flags().StringVarP(&searchDir, "dir", "d", ".", "Directory to search in")
}
