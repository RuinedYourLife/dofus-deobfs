package utils

import (
	"log/slog"
)

// FindEnumBasedMatches finds messages that have matching enum definitions
func FindEnumBasedMatches(obfuscated, unobfuscated *Descriptor, logger *slog.Logger) []MessageMatch {
	var matches []MessageMatch
	var totalObfuscatedWithEnums int
	var matchedMessages = make(map[string]bool)

	// First count how many messages have enums
	for _, obsMsg := range obfuscated.MessageType {
		if len(obsMsg.EnumType) > 0 {
			totalObfuscatedWithEnums++
		}
	}

	// For each obfuscated message that has enums
	for _, obsMsg := range obfuscated.MessageType {
		if len(obsMsg.EnumType) == 0 {
			continue
		}

		// Look for matches in unobfuscated messages
		for _, unobsMsg := range unobfuscated.MessageType {
			if len(unobsMsg.EnumType) != len(obsMsg.EnumType) {
				continue
			}

			// Check if all enums match
			allEnumsMatch := true
			for i, obsEnum := range obsMsg.EnumType {
				if !compareEnums(obsEnum, unobsMsg.EnumType[i]) {
					allEnumsMatch = false
					break
				}
			}

			if allEnumsMatch {
				matches = append(matches, MessageMatch{
					ObfuscatedMsg:  obsMsg.Name,
					ObfuscatedFile: obsMsg.SourceFile,
					OriginalMsg:    unobsMsg.Name,
					OriginalFile:   unobsMsg.SourceFile,
					MatchPercent:   100,
				})
				logger.Debug("found enum-based match",
					"obfuscated", obsMsg.Name,
					"obfuscated_file", obsMsg.SourceFile,
					"original", unobsMsg.Name,
					"original_file", unobsMsg.SourceFile,
				)
				for _, enum := range obsMsg.EnumType {
					logger.Debug("matching enum",
						"name", enum.Name,
						"values", enum.Value,
					)
				}
				matchedMessages[obsMsg.Name] = true
				break
			}
		}
	}

	logger.Info("enum matching complete",
		"matches", len(matches),
		"total_with_enums", totalObfuscatedWithEnums,
	)

	if len(matches) < totalObfuscatedWithEnums {
		logger.Warn("some messages with enums weren't matched")
		for _, obsMsg := range obfuscated.MessageType {
			if len(obsMsg.EnumType) > 0 && !matchedMessages[obsMsg.Name] {
				logger.Warn("unmatched message",
					"name", obsMsg.Name,
					"enums", obsMsg.EnumType,
				)
			}
		}
	}

	return matches
}

// Returns true if both enum types have exactly the same values
func compareEnums(obfs, unobfs EnumType) bool {
	if len(obfs.Value) != len(unobfs.Value) {
		return false
	}

	// Create maps of name->number for both enums
	obfsMap := make(map[string]int)
	unobsMap := make(map[string]int)

	for _, v := range obfs.Value {
		obfsMap[v.Name] = v.Number
	}
	for _, v := range unobfs.Value {
		unobsMap[v.Name] = v.Number
	}

	// Check that every value matches exactly
	for name, number := range obfsMap {
		if unobsNumber, exists := unobsMap[name]; !exists || unobsNumber != number {
			return false
		}
	}

	return true
}
