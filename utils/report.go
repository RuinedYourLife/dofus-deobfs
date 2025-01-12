package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func GenerateMatchReport(matches []MessageMatch, outputFile string) error {
	var report strings.Builder

	report.WriteString("Message Matches Report\n")
	report.WriteString("======================\n\n")

	// Sort matches for consistent output
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].ObfuscatedFile != matches[j].ObfuscatedFile {
			return matches[i].ObfuscatedFile < matches[j].ObfuscatedFile
		}
		return matches[i].ObfuscatedMsg < matches[j].ObfuscatedMsg
	})

	// Calculate column widths
	var maxObfsMsg, maxOrigMsg, maxOrigFile int
	for _, match := range matches {
		maxObfsMsg = max(maxObfsMsg, len(match.ObfuscatedMsg))
		maxOrigMsg = max(maxOrigMsg, len(match.OriginalMsg))
		maxOrigFile = max(maxOrigFile, len(filepath.Base(match.OriginalFile)))
	}

	// Write header
	format := fmt.Sprintf("%%-%ds  →  %%-%ds  %%-%ds  [conf: %%%d.2f%%%%]\n",
		maxObfsMsg, maxOrigMsg, maxOrigFile, 6)

	report.WriteString(fmt.Sprintf(format,
		"Obf",
		"Orig",
		"File",
		0.0,
	))

	// Write separator
	totalWidth := maxObfsMsg + maxOrigMsg + maxOrigFile + 23 // 20 for spacing and symbols
	report.WriteString(strings.Repeat("-", totalWidth) + "\n")

	// Write matches
	for _, match := range matches {
		if len(match.Alternatives) > 0 {
			// For uncertain matches, list all possibilities as alternatives
			allPossibilities := append([]string{match.OriginalMsg}, match.Alternatives...)
			report.WriteString(fmt.Sprintf(format,
				match.ObfuscatedMsg,
				"???", // Show uncertainty in main match
				"???", // Don't show file when uncertain
				match.MatchPercent,
			))
			report.WriteString(fmt.Sprintf("    Possible matches: %s\n",
				strings.Join(allPossibilities, ", ")))
		} else {
			// For definitive matches
			report.WriteString(fmt.Sprintf(format,
				match.ObfuscatedMsg,
				match.OriginalMsg,
				filepath.Base(match.OriginalFile),
				match.MatchPercent,
			))
		}
	}

	report.WriteString(fmt.Sprintf("\nTotal matches: %d\n", len(matches)))

	return os.WriteFile(outputFile, []byte(report.String()), 0644)
}
