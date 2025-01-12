package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func GenerateEnumMatchReport(matches []MessageMatch, outputFile string) error {
	var report strings.Builder

	report.WriteString("Enum-Based Message Matches Report\n")
	report.WriteString("===============================\n\n")

	// Group matches by original file
	fileMatches := make(map[string][]MessageMatch)
	for _, match := range matches {
		fileMatches[match.OriginalFile] = append(fileMatches[match.OriginalFile], match)
	}

	// Sort files for consistent output
	var files []string
	for file := range fileMatches {
		files = append(files, file)
	}
	sort.Strings(files)

	// Write matches grouped by file
	for _, file := range files {
		report.WriteString(fmt.Sprintf("\nFile: %s\n", file))
		report.WriteString(strings.Repeat("-", len(file)+6) + "\n")

		matches := fileMatches[file]
		sort.Slice(matches, func(i, j int) bool {
			return matches[i].ObfuscatedMsg < matches[j].ObfuscatedMsg
		})

		for _, match := range matches {
			report.WriteString(fmt.Sprintf("%s (%s) -> %s (confidence: %.0f%%)\n",
				match.ObfuscatedMsg,
				filepath.Base(match.ObfuscatedFile),
				match.OriginalMsg,
				match.MatchPercent,
			))
		}
	}

	report.WriteString(fmt.Sprintf("\nTotal matches: %d across %d files\n",
		len(matches),
		len(fileMatches),
	))

	return os.WriteFile(outputFile, []byte(report.String()), 0644)
}
