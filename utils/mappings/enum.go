package mappings

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/ruinedyourlife/deobfs/utils"
)

// FindEnumBasedMatches finds messages that have matching enum definitions
func FindEnumBasedMatches(obfuscated, unobfuscated *utils.Descriptor, logger *slog.Logger) []utils.MessageMatch {
	// Initialize progress at start
	utils.GlobalProgress.Init(len(obfuscated.MessageType))

	var matches []utils.MessageMatch
	var totalObfuscatedWithEnums int
	var matchedMessages = make(map[string]bool)

	// Count messages with enums
	for _, obsMsg := range obfuscated.MessageType {
		if len(getAllEnums(obsMsg, "")) > 0 {
			totalObfuscatedWithEnums++
		}
	}

	// For each obfuscated message
	for _, obsMsg := range obfuscated.MessageType {
		obfsEnums := getAllEnums(obsMsg, "")
		if len(obfsEnums) == 0 {
			continue
		}

		// For each unobfuscated message
		for _, unobsMsg := range unobfuscated.MessageType {
			unobsEnums := getAllEnums(unobsMsg, "")

			var enumMatches []utils.EnumMatch
			var allEnumsMatched bool = true

			// Try to match each enum and find their parent messages
			for obfsPath, obfsEnum := range obfsEnums {
				matched := false
				var bestMatch utils.EnumMatch
				var bestConfidence float64

				for unobsPath, unobsEnum := range unobsEnums {
					if isMatch, confidence := compareEnums(obfsEnum, unobsEnum); isMatch {
						// Get top-level messages containing these enums
						obfsParent := getTopLevelMessage(obsMsg, strings.Split(obfsPath, ".")[0])
						unobsParent := getTopLevelMessage(unobsMsg, strings.Split(unobsPath, ".")[0])

						if confidence > bestConfidence {
							bestMatch = utils.EnumMatch{
								ObfuscatedEnum: obfsPath,
								OriginalEnum:   unobsPath,
								Values:         formatEnumValues(obfsEnum.Value),
								Confidence:     confidence,
							}
							bestConfidence = confidence
							matched = true
						}

						logger.Debug("found matching enum in messages",
							"obfuscated_msg", obfsParent,
							"original_msg", unobsParent,
							"enum_match", fmt.Sprintf("%s -> %s", obfsPath, unobsPath),
						)
					}
				}

				if matched {
					enumMatches = append(enumMatches, bestMatch)
				} else {
					allEnumsMatched = false
				}
			}

			// If we found matches, match the top-level messages
			if allEnumsMatched && len(enumMatches) > 0 {
				// Calculate average confidence
				var totalConfidence float64
				for _, enumMatch := range enumMatches {
					totalConfidence += enumMatch.Confidence
				}
				averageConfidence := totalConfidence / float64(len(enumMatches))

				match := utils.MessageMatch{
					ObfuscatedMsg:  obsMsg.Name,
					ObfuscatedFile: obsMsg.SourceFile,
					OriginalMsg:    unobsMsg.Name,
					OriginalFile:   unobsMsg.SourceFile,
					MatchPercent:   averageConfidence,
					EnumMatches:    enumMatches,
				}
				matches = append(matches, match)
				matchedMessages[obsMsg.Name] = true

				logger.Debug("found top-level message match",
					"obfuscated", obsMsg.Name,
					"original", unobsMsg.Name,
				)

				for _, enumMatch := range enumMatches {
					logger.Debug("matching enum",
						"obfuscated_enum", enumMatch.ObfuscatedEnum,
						"original_enum", enumMatch.OriginalEnum,
						"values", enumMatch.Values,
					)
				}
				break
			}
		}
	}

	// Update progress when we find matches
	utils.GlobalProgress.AddMatches(len(matches))

	// Enhanced summary logging
	logger.Info("enum matching summary",
		"obfuscated_with_enums", totalObfuscatedWithEnums,
		"enum_matches_found", len(matches),
		"matching_progress", fmt.Sprintf("%.1f%%", utils.GlobalProgress.GetProgress()),
	)

	// Log unmatched messages
	if len(matches) < totalObfuscatedWithEnums {
		for _, obsMsg := range obfuscated.MessageType {
			if obfsEnums := getAllEnums(obsMsg, ""); len(obfsEnums) > 0 && !matchedMessages[obsMsg.Name] {
				logger.Debug("unmatched message",
					"name", obsMsg.Name,
					"enums", formatEnumPaths(obfsEnums),
				)
			}
		}
	}

	return matches
}

// Returns true if both enum types have matching values, with a confidence score
func compareEnums(obfs, unobfs utils.EnumType) (bool, float64) {
	// Create maps of name->number for both enums
	obfsMap := make(map[string]int)
	unobsMap := make(map[string]int)

	for _, v := range obfs.Value {
		obfsMap[v.Name] = v.Number
	}
	for _, v := range unobfs.Value {
		unobsMap[v.Name] = v.Number
	}

	// Count matching values
	matchingValues := 0
	for name, number := range obfsMap {
		if unobsNumber, exists := unobsMap[name]; exists && unobsNumber == number {
			matchingValues++
		}
	}

	// Calculate confidence based on matching ratio
	smallerSize := len(obfsMap)
	if len(unobsMap) < smallerSize {
		smallerSize = len(unobsMap)
	}

	// If all values in the smaller enum match, consider it a match
	if matchingValues == smallerSize {
		confidence := float64(matchingValues) / float64(max(len(obfsMap), len(unobsMap))) * 100
		return true, confidence
	}

	return false, 0
}

// Helper function to get all enums in a message and its nested messages
func getAllEnums(msg utils.MessageType, parentPath string) map[string]utils.EnumType {
	enums := make(map[string]utils.EnumType)

	// Add direct enums with proper parent path
	for _, enum := range msg.EnumType {
		path := parentPath
		if path == "" {
			path = msg.Name
		}
		enums[path+"."+enum.Name] = enum
	}

	// Add nested message enums with proper hierarchy
	for _, nested := range msg.NestedType {
		nestedPath := parentPath
		if nestedPath == "" {
			nestedPath = msg.Name
		}
		nestedPath = nestedPath + "." + nested.Name
		for path, enum := range getAllEnums(nested, nestedPath) {
			enums[path] = enum
		}
	}

	return enums
}

// Helper to get the top-level message containing an enum
func getTopLevelMessage(msg utils.MessageType, enumPath string) string {
	parts := strings.Split(enumPath, ".")
	if len(parts) < 2 {
		return ""
	}
	topMsg := parts[0] // First part should be the top-level message name

	// If this is the top message, check if it owns the enum
	if msg.Name == topMsg {
		return msg.Name
	}

	// Check nested messages
	for _, nested := range msg.NestedType {
		if found := getTopLevelMessage(nested, enumPath); found != "" {
			return msg.Name // Return the parent message name
		}
	}

	return ""
}

func formatEnumValues(values []utils.EnumValue) []string {
	result := make([]string, len(values))
	for i, v := range values {
		result[i] = fmt.Sprintf("%s=%d", v.Name, v.Number)
	}
	return result
}

func formatEnumPaths(enums map[string]utils.EnumType) string {
	var parts []string
	for path, enum := range enums {
		values := formatEnumValues(enum.Value)
		parts = append(parts, fmt.Sprintf("%s: [%s]", path, strings.Join(values, ", ")))
	}
	return strings.Join(parts, " | ")
}
