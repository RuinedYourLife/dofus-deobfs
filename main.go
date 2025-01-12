package main

import (
	"flag"
	"os"

	"github.com/ruinedyourlife/deobfs/utils"
	"github.com/ruinedyourlife/deobfs/utils/mappings"
)

func main() {
	// Add command line flags for log level
	logLevel := flag.String("log", "info", "log level (debug, info, warn, error)")
	flag.Parse()

	// Convert string level to LogLevel
	var level utils.LogLevel
	switch *logLevel {
	case "debug":
		level = utils.LevelDebug
	case "info":
		level = utils.LevelInfo
	case "warn":
		level = utils.LevelWarn
	case "error":
		level = utils.LevelError
	default:
		level = utils.LevelInfo
	}

	logger := utils.InitLogger(level)

	// Use protodec to generate all the proto files which you can put
	// in the protos/decompiled directory
	config := utils.Config{
		SourceDir: "protos/decompiled",
		OutputDir: "protos/filtered",
		AssembliesOfInterest: []string{
			"Ankama.Dofus.Protocol.Connection",
			"Ankama.Dofus.Protocol.Game",
		},
	}

	if err := utils.FilterProtoFiles(config); err != nil {
		logger.Error("error filtering proto files", "error", err)
	}

	// Example: only process specific files
	filter := []string{}
	// Or leave empty for all files
	// filter := []string{}

	logger.Info("loading and parsing proto files...")

	obfuscated, err := utils.LoadAndParseProtos("protos/filtered", filter, logger)
	if err != nil {
		logger.Error("error loading obfuscated protos", "error", err)
		os.Exit(1)
	}

	unobfuscated, err := utils.LoadAndParseProtos("protos/clear", filter, logger)
	if err != nil {
		logger.Error("error loading unobfuscated protos", "error", err)
		os.Exit(1)
	}

	// 1. Find matches based on enum values
	enumMatches := mappings.FindEnumBasedMatches(obfuscated, unobfuscated, logger)

	// 2. Find matches based on strict message structures (1-1 match)
	structureMatches := mappings.FindStrictStructureBasedMatches(obfuscated, unobfuscated, enumMatches, logger)

	// Generate reports
	if err := utils.GenerateMatchReport(enumMatches, "reports/enum_matches.txt"); err != nil {
		logger.Error("failed to generate enum matches report", "error", err)
	}

	if err := utils.GenerateMatchReport(structureMatches, "reports/structure_matches.txt"); err != nil {
		logger.Error("failed to generate structure matches report", "error", err)
	}
}
