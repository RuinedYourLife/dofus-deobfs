package mappings

import (
	"fmt"
	"log/slog"
	"math"

	"github.com/ruinedyourlife/deobfs/utils"
)

// FindStrictStructureBasedMatches finds messages that have matching structure/fields
func FindStrictStructureBasedMatches(
	obfuscated, unobfuscated *utils.Descriptor,
	enumMatches []utils.MessageMatch,
	logger *slog.Logger,
) []utils.MessageMatch {
	// We’ll store final structure-based matches here
	var matches []utils.MessageMatch

	// Keep track of which messages are already matched (including those from enumMatches)
	matchedObfuscated := make(map[string]bool)
	matchedUnobfuscated := make(map[string]bool)

	// Mark messages from enum matching as already matched
	for _, em := range enumMatches {
		matchedObfuscated[em.ObfuscatedMsg] = true
		matchedUnobfuscated[em.OriginalMsg] = true
	}

	// Build slices of unmatched messages
	var unmatchedObs []utils.MessageType
	var unmatchedUnobs []utils.MessageType

	for _, msg := range obfuscated.MessageType {
		if !matchedObfuscated[msg.Name] {
			unmatchedObs = append(unmatchedObs, msg)
		}
	}
	for _, msg := range unobfuscated.MessageType {
		if !matchedUnobfuscated[msg.Name] {
			unmatchedUnobs = append(unmatchedUnobs, msg)
		}
	}

	// Count how many we started with—useful for summary logging
	startingUnmatched := len(unmatchedObs)

	// Iteratively peel off single-candidate matches
	somethingChanged := true
	passes := 0
	for somethingChanged {
		passes++
		somethingChanged = false

		// We'll keep track of newly matched in this pass
		newlyMatchedObs := make([]string, 0)

		// Go through each unmatched obfuscated message
		for _, obsMsg := range unmatchedObs {
			if matchedObfuscated[obsMsg.Name] {
				continue
			}

			// Find all possible "perfect" matches among unmatched unobs
			var candidates []utils.MessageType
			for _, unobsMsg := range unmatchedUnobs {
				if matchedUnobfuscated[unobsMsg.Name] {
					continue
				}

				// For 100% strict matching
				if isPerfectStructureMatch(obsMsg, unobsMsg) {
					candidates = append(candidates, unobsMsg)
				}
			}

			// If exactly one perfect match, we accept it
			if len(candidates) == 1 {
				matched := candidates[0]
				matchedObfuscated[obsMsg.Name] = true
				matchedUnobfuscated[matched.Name] = true
				newlyMatchedObs = append(newlyMatchedObs, obsMsg.Name)

				// Because compareMessageStructures returns a confidence
				// we'll retrieve it again for logging/storing
				_, confidence := compareMessageStructures(obsMsg, matched)

				match := utils.MessageMatch{
					ObfuscatedMsg:  obsMsg.Name,
					ObfuscatedFile: obsMsg.SourceFile,
					OriginalMsg:    matched.Name,
					OriginalFile:   matched.SourceFile,
					MatchPercent:   confidence, // should be 100
				}
				matches = append(matches, match)

				logger.Debug("structure-based match",
					"obfuscated", obsMsg.Name,
					"original", matched.Name,
					"confidence", confidence,
				)

				somethingChanged = true
			}
		}

		// Remove newly matched obs messages from unmatchedObs
		if somethingChanged && len(newlyMatchedObs) > 0 {
			var tempObs []utils.MessageType
			for _, oMsg := range unmatchedObs {
				if !matchedObfuscated[oMsg.Name] {
					tempObs = append(tempObs, oMsg)
				}
			}
			unmatchedObs = tempObs

			// Also remove matched unobs
			var tempUnobs []utils.MessageType
			for _, uMsg := range unmatchedUnobs {
				if !matchedUnobfuscated[uMsg.Name] {
					tempUnobs = append(tempUnobs, uMsg)
				}
			}
			unmatchedUnobs = tempUnobs
		}
	}

	// Update progress when we find new matches
	utils.GlobalProgress.AddMatches(len(matches))

	// After no more single-candidate matches remain, we can do a summary
	strictMatches := len(matches)
	logger.Info("strict structure matching summary",
		"initial_unmatched_obfuscated", startingUnmatched,
		"strict_matches_found", strictMatches,
		"passes_needed", passes,
		"matching_progress", fmt.Sprintf("%.1f%%", utils.GlobalProgress.GetProgress()),
	)

	// Return only the strict matches. The rest remain unmatched/ambiguous.
	return matches
}

// Returns true if both messages have matching structure, with a confidence score
func compareMessageStructures(obfs, unobs utils.MessageType) (bool, float64) {
	// Skip messages with no fields
	if len(obfs.Field) == 0 || len(unobs.Field) == 0 {
		return false, 0
	}

	// Compare basic structure
	matchScore := 0.0
	totalChecks := 0.0

	// Check field count similarity
	fieldCountDiff := float64(math.Abs(float64(len(obfs.Field) - len(unobs.Field))))
	fieldCountScore := 1.0 - (fieldCountDiff / float64(math.Max(float64(len(obfs.Field)), float64(len(unobs.Field)))))
	matchScore += fieldCountScore
	totalChecks++

	// Check field types in order
	matchingFields := 0
	maxFields := min(len(obfs.Field), len(unobs.Field))
	for i := 0; i < maxFields; i++ {
		obfsField := obfs.Field[i]
		unobsField := unobs.Field[i]

		// Compare field properties
		if compareFields(obfsField, unobsField) {
			matchingFields++
		}
	}

	if maxFields > 0 {
		fieldTypeScore := float64(matchingFields) / float64(maxFields)
		matchScore += fieldTypeScore
		totalChecks++
	}

	// Check oneof count and structure
	if len(obfs.OneOfDecl) > 0 || len(unobs.OneOfDecl) > 0 {
		oneofCountDiff := float64(math.Abs(float64(len(obfs.OneOfDecl) - len(unobs.OneOfDecl))))
		oneofScore := 1.0 - (oneofCountDiff / float64(max(len(obfs.OneOfDecl), len(unobs.OneOfDecl))))
		matchScore += oneofScore
		totalChecks++

		// Compare oneof fields
		for i := 0; i < min(len(obfs.OneOfDecl), len(unobs.OneOfDecl)); i++ {
			obfsOneofFields := getOneofFields(obfs, i)
			unobsOneofFields := getOneofFields(unobs, i)

			oneofFieldMatch := compareOneofFields(obfsOneofFields, unobsOneofFields)
			matchScore += oneofFieldMatch
			totalChecks++
		}
	}

	// Check nested message count and structure
	if len(obfs.NestedType) > 0 || len(unobs.NestedType) > 0 {
		nestedCountDiff := float64(math.Abs(float64(len(obfs.NestedType) - len(unobs.NestedType))))
		nestedScore := 1.0 - (nestedCountDiff / float64(max(len(obfs.NestedType), len(unobs.NestedType))))
		matchScore += nestedScore
		totalChecks++
	}

	// Calculate final confidence
	if totalChecks == 0 {
		return false, 0
	}

	confidence := (matchScore / totalChecks) * 100

	// Only consider it a match if confidence is above threshold
	return confidence >= 80, confidence
}

// Wrapper to check if a structure match is perfect
func isPerfectStructureMatch(obfs, unobs utils.MessageType) bool {
	isMatch, confidence := compareMessageStructures(obfs, unobs)
	return isMatch && confidence == 100
}

// Helper functions
func compareFields(obfs, unobs utils.Field) bool {
	// Compare basic field properties
	if obfs.Label != unobs.Label {
		return false
	}

	// Compare types, handling both primitive and message types
	return compareTypes(obfs.Type, unobs.Type)
}

func compareTypes(obfsType, unobsType string) bool {
	// Handle primitive types
	primitiveTypes := map[string][]string{
		"int32":  {"int32"},
		"int64":  {"int64"},
		"string": {"string"},
		"bool":   {"bool"},
	}

	for _, compatTypes := range primitiveTypes {
		if contains(compatTypes, obfsType) && contains(compatTypes, unobsType) {
			return true
		}
	}

	return false
}

func getOneofFields(msg utils.MessageType, oneofIndex int) []utils.Field {
	var fields []utils.Field
	for _, field := range msg.Field {
		if field.OneOfIndex != nil && *field.OneOfIndex == oneofIndex {
			fields = append(fields, field)
		}
	}
	return fields
}

func compareOneofFields(obfsFields, unobsFields []utils.Field) float64 {
	if len(obfsFields) == 0 || len(unobsFields) == 0 {
		return 0
	}

	matchingFields := 0
	for _, obfsField := range obfsFields {
		for _, unobsField := range unobsFields {
			if compareFields(obfsField, unobsField) {
				matchingFields++
				break
			}
		}
	}

	return float64(matchingFields) / float64(max(len(obfsFields), len(unobsFields)))
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
