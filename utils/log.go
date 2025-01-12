package utils

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/fatih/color"
)

var Logger *slog.Logger

type LogLevel slog.Level

const (
	LevelDebug = LogLevel(slog.LevelDebug)
	LevelInfo  = LogLevel(slog.LevelInfo)
	LevelWarn  = LogLevel(slog.LevelWarn)
	LevelError = LogLevel(slog.LevelError)
)

const maxEnumValuesLength = 80

func truncateEnumValues(values string) string {
	if len(values) <= maxEnumValuesLength {
		return values
	}
	return values[:maxEnumValuesLength] + "..."
}

type PrettyHandler struct {
	slog.Handler
	l *slog.Logger
}

func (h *PrettyHandler) Handle(ctx context.Context, r slog.Record) error {
	// Get level prefix
	level := ""
	switch r.Level {
	case slog.LevelDebug:
		level = color.BlueString("DBG")
	case slog.LevelInfo:
		level = color.GreenString("INF")
	case slog.LevelWarn:
		level = color.YellowString("WRN")
	case slog.LevelError:
		level = color.RedString("ERR")
	}

	// Get all attributes
	var orderedAttrs []struct{ k, v string }
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "level" {
			return true
		}
		orderedAttrs = append(orderedAttrs, struct{ k, v string }{a.Key, a.Value.String()})
		return true
	})

	// Format based on message type
	var output string
	switch msg := r.Message; msg {
	case "found matching enum in messages":
		obfsMsg, origMsg, enumMatch := "", "", ""
		for _, attr := range orderedAttrs {
			switch attr.k {
			case "obfuscated_msg":
				obfsMsg = color.GreenString(attr.v)
			case "original_msg":
				origMsg = color.GreenString(attr.v)
			case "enum_match":
				enumMatch = color.YellowString(attr.v)
			}
		}
		output = fmt.Sprintf("%s found matching enum between messages: %s -> %s (%s)",
			level, obfsMsg, origMsg, enumMatch)

	case "found top-level message match":
		obfs, orig := "", ""
		for _, attr := range orderedAttrs {
			switch attr.k {
			case "obfuscated":
				obfs = color.GreenString(attr.v)
			case "original":
				orig = color.GreenString(attr.v)
			}
		}
		output = fmt.Sprintf("%s found top-level message match: %s -> %s",
			level, obfs, orig)

	case "matching enum":
		obfsEnum, origEnum, values := "", "", ""
		for _, attr := range orderedAttrs {
			switch attr.k {
			case "obfuscated_enum":
				obfsEnum = color.YellowString(attr.v)
			case "original_enum":
				origEnum = color.YellowString(attr.v)
			case "values":
				values = truncateEnumValues(attr.v)
			}
		}
		output = fmt.Sprintf("%s     matching enum: %s -> %s with values: %s",
			level, obfsEnum, origEnum, values)

	case "matching summary":
		var withEnums, found string
		var progress float64
		for _, attr := range orderedAttrs {
			switch attr.k {
			case "obfuscated_with_enums":
				withEnums = color.YellowString(attr.v)
			case "enum_matches_found":
				found = color.GreenString(attr.v)
			case "matching_progress":
				progress, _ = strconv.ParseFloat(strings.TrimSuffix(attr.v, "%"), 64)
			}
		}

		progressBar := createProgressBar(progress)
		output = fmt.Sprintf(`%s Matching Summary:
    Enum Matches:
        Messages with enums: %s
        Matches found:       %s
    Progress: %s %.1f%%`,
			level,
			withEnums,
			found,
			progressBar,
			progress,
		)

	case "unmatched message":
		name, enums := "", ""
		for _, attr := range orderedAttrs {
			switch attr.k {
			case "name":
				name = color.RedString(attr.v)
			case "enums":
				enums = truncateEnumValues(attr.v)
			}
		}
		output = fmt.Sprintf("%s     %s (enum values: %s)",
			level, name, enums)

	default:
		output = fmt.Sprintf("%s %s", level, msg)
		for _, attr := range orderedAttrs {
			output += fmt.Sprintf(" %s=%s",
				color.New(color.Bold).Sprint(attr.k),
				strings.TrimSpace(attr.v),
			)
		}
	}

	// Write to output
	_, err := fmt.Fprintln(os.Stdout, output)
	return err
}

func InitLogger(level LogLevel) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level: slog.Level(level),
	}

	handler := slog.NewTextHandler(os.Stdout, opts)
	prettyHandler := &PrettyHandler{handler, nil}
	Logger = slog.New(prettyHandler)
	prettyHandler.l = Logger
	slog.SetDefault(Logger)
	return Logger
}

// Helper to create a progress bar
func createProgressBar(percent float64) string {
	width := 30
	completed := int(percent * float64(width) / 100)

	bar := strings.Builder{}
	bar.WriteString("[")

	// Add completed portion
	bar.WriteString(color.GreenString(strings.Repeat("=", completed)))

	// Add remaining portion
	if completed < width {
		bar.WriteString(color.HiBlackString(strings.Repeat("-", width-completed)))
	}

	bar.WriteString("]")
	return bar.String()
}
