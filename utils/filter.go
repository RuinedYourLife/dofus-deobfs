package utils

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config holds the configuration for the proto file filtering
type Config struct {
	SourceDir            string
	OutputDir            string
	AssembliesOfInterest []string
}

// FilterProtoFiles processes proto files according to the given configuration
func FilterProtoFiles(config Config) error {
	// Check if source directory exists
	if _, err := os.Stat(config.SourceDir); os.IsNotExist(err) {
		return fmt.Errorf("source directory %s does not exist. Please create it first and use protodec to generate the proto files", config.SourceDir)
	}

	// Check if directory is empty
	entries, err := os.ReadDir(config.SourceDir)
	if err != nil {
		return fmt.Errorf("error reading source directory: %v", err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("source directory %s is empty. Please use protodec to generate the proto files first", config.SourceDir)
	}

	return filepath.Walk(config.SourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("[-] error accessing path %s: %v\n", path, err)
			return nil
		}

		// Process only .proto files
		if filepath.Ext(info.Name()) == ".proto" {
			if shouldIncludeFile(path, config.AssembliesOfInterest) {
				destination := filepath.Join(config.OutputDir, info.Name())
				err := copyFile(path, destination)
				if err != nil {
					fmt.Printf("[-] error copying file %s: %v\n", path, err)
				}
			}
		}
		return nil
	})
}

func shouldIncludeFile(path string, assembliesOfInterest []string) bool {
	file, err := os.Open(path)
	if err != nil {
		fmt.Printf("[-] error opening file %s: %v\n", path, err)
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		for _, assembly := range assembliesOfInterest {
			if strings.Contains(line, assembly) {
				return true
			}
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("[-] error reading file %s: %v\n", path, err)
		return false
	}

	return false
}

func copyFile(source, destination string) error {
	srcFile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer destFile.Close()

	writer := bufio.NewWriter(destFile)
	scanner := bufio.NewScanner(srcFile)

	// Flag to track if we've written the syntax line
	syntaxWritten := false

	for scanner.Scan() {
		line := scanner.Text()

		// Skip comment lines
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}

		// Handle syntax line
		if strings.HasPrefix(strings.TrimSpace(line), "syntax") {
			if !syntaxWritten {
				_, err := writer.WriteString("syntax = \"proto3\";\n\n")
				if err != nil {
					return err
				}
				syntaxWritten = true
			}
			continue
		}

		// If we haven't written the syntax line yet and this is the first content line,
		// insert it first
		if !syntaxWritten {
			_, err := writer.WriteString("syntax = \"proto3\";\n\n")
			if err != nil {
				return err
			}
			syntaxWritten = true
		}

		// Write the current line with original indentation
		_, err := writer.WriteString(line + "\n")
		if err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return writer.Flush()
}
