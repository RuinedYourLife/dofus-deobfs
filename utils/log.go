package utils

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
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
	case "found enum-based match":
		obfs, obfsFile, orig, origFile := "", "", "", ""
		for _, attr := range orderedAttrs {
			switch attr.k {
			case "obfuscated":
				obfs = color.GreenString(attr.v)
			case "obfuscated_file":
				obfsFile = color.BlueString(filepath.Base(attr.v))
			case "original":
				orig = color.GreenString(attr.v)
			case "original_file":
				origFile = color.BlueString(filepath.Base(attr.v))
			}
		}
		output = fmt.Sprintf("%s found enum-based match: %s (%s) -> %s (%s)",
			level, obfs, obfsFile, orig, origFile)

	case "matching enum":
		name, values := "", ""
		for _, attr := range orderedAttrs {
			switch attr.k {
			case "name":
				name = color.YellowString(attr.v)
			case "values":
				values = truncateEnumValues(attr.v)
			}
		}
		output = fmt.Sprintf("%s     matching enum: %s with values: %s",
			level, name, values)

	case "enum matching complete":
		matches, total := "", ""
		for _, attr := range orderedAttrs {
			switch attr.k {
			case "matches":
				matches = color.GreenString(attr.v)
			case "total_with_enums":
				total = color.GreenString(attr.v)
			}
		}
		output = fmt.Sprintf("%s found %s enum-based matches out of %s messages with enums",
			level, matches, total)

	case "some messages with enums weren't matched":
		output = fmt.Sprintf("%s messages with enums that weren't matched:", level)

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
