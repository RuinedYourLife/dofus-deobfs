package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"log/slog"

	"github.com/fatih/color"
)

type EnumMatch struct {
	ObfuscatedEnum string   // Full path like "iqe.ipz"
	OriginalEnum   string   // Full path like "ExchangeCraftResultEvent.CraftResult"
	Values         []string // For logging/debugging
	Confidence     float64  // Store the confidence score
}

type MessageMatch struct {
	ObfuscatedMsg  string
	ObfuscatedFile string
	OriginalMsg    string
	OriginalFile   string
	MatchPercent   float64
	EnumMatches    []EnumMatch
}

type EnumValue struct {
	Name   string `json:"name"`
	Number int    `json:"number"`
}

type EnumType struct {
	Name  string      `json:"name"`
	Value []EnumValue `json:"value"`
}

type Field struct {
	Name       string `json:"name"`
	Number     int    `json:"number"`
	Label      string `json:"label"`
	Type       string `json:"type"`
	TypeName   string `json:"typeName"`
	OneOfIndex *int   `json:"oneofIndex"`
}

type OneOfDecl struct {
	Name string `json:"name"`
}

type MessageType struct {
	Name       string        `json:"name"`
	Field      []Field       `json:"field"`
	NestedType []MessageType `json:"nestedType"`
	EnumType   []EnumType    `json:"enumType"`
	OneOfDecl  []OneOfDecl   `json:"oneofDecl"`
	SourceFile string        `json:"-"`
}

type Descriptor struct {
	Name        string        `json:"name"`
	Package     string        `json:"package"`
	Dependency  []string      `json:"dependency"`
	MessageType []MessageType `json:"messageType"`
	EnumType    []EnumType    `json:"enumType"`
	Syntax      string        `json:"syntax"`
}

func LoadAndParseProtos(dir string, filter []string, logger *slog.Logger) (*Descriptor, error) {
	var desc Descriptor
	fileCount := 0

	// Create a map for faster lookup if we have filters
	filterMap := make(map[string]bool)
	for _, f := range filter {
		filterMap[f] = true
	}

	logger.Info(fmt.Sprintf("loading proto files from %s", color.BlueString(dir)))
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(info.Name(), ".proto") {
			// Skip if we have filters and this file isn't in the list
			if len(filterMap) > 0 {
				if !filterMap[info.Name()] {
					return nil
				}
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			fileDesc, err := ParseProtoFile(string(content))
			if err != nil {
				return fmt.Errorf("parsing %s: %w", path, err)
			}

			// Set source file for all messages in this file
			for i := range fileDesc.MessageType {
				fileDesc.MessageType[i].SourceFile = path
			}

			// debugPrintDescriptor(fileDesc)
			desc.MessageType = append(desc.MessageType, fileDesc.MessageType...)
			fileCount++
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	logger.Info(fmt.Sprintf("parsed %s files & %s messages",
		color.GreenString(strconv.Itoa(fileCount)),
		color.GreenString(strconv.Itoa(countTotalMessages(desc.MessageType))),
	))
	return &desc, nil
}

func ParseProtoFile(content string) (*Descriptor, error) {
	var desc Descriptor
	var currentMsg *MessageType
	var currentEnum *EnumType
	var currentOneofIndex *int
	var parentMsgs []*MessageType
	var nestLevel int

	lines := strings.Split(content, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		// Track opening braces
		if strings.Contains(line, "{") {
			nestLevel++
		}

		// Handle closing braces
		if line == "}" {
			nestLevel--
			if currentEnum != nil {
				currentEnum = nil
			} else if currentOneofIndex != nil && nestLevel == 1 {
				currentOneofIndex = nil
			} else if currentMsg != nil {
				if len(parentMsgs) > 0 {
					currentMsg = parentMsgs[len(parentMsgs)-1]
					parentMsgs = parentMsgs[:len(parentMsgs)-1]
				} else if nestLevel == 0 {
					currentMsg = nil
				}
			}
			continue
		}

		if strings.HasPrefix(line, "message ") {
			name := strings.TrimSuffix(strings.TrimPrefix(line, "message "), " {")
			msg := MessageType{Name: name}
			if currentMsg == nil {
				desc.MessageType = append(desc.MessageType, msg)
				currentMsg = &desc.MessageType[len(desc.MessageType)-1]
			} else {
				parentMsgs = append(parentMsgs, currentMsg)
				currentMsg.NestedType = append(currentMsg.NestedType, msg)
				currentMsg = &currentMsg.NestedType[len(currentMsg.NestedType)-1]
			}
			continue
		}

		if strings.HasPrefix(line, "enum ") {
			name := strings.TrimSpace(strings.TrimPrefix(line, "enum "))
			name = strings.TrimSuffix(name, "{")
			enum := EnumType{Name: name}
			if currentMsg != nil {
				currentMsg.EnumType = append(currentMsg.EnumType, enum)
				currentEnum = &currentMsg.EnumType[len(currentMsg.EnumType)-1]
			} else {
				desc.EnumType = append(desc.EnumType, enum)
				currentEnum = &desc.EnumType[len(desc.EnumType)-1]
			}
			continue
		}

		// Parse oneof definitions
		if strings.HasPrefix(line, "oneof ") {
			if currentMsg != nil {
				name := strings.TrimSpace(strings.TrimPrefix(line, "oneof "))
				name = strings.TrimSpace(strings.TrimSuffix(name, "{"))
				idx := len(currentMsg.OneOfDecl)
				currentMsg.OneOfDecl = append(currentMsg.OneOfDecl, OneOfDecl{Name: name})
				currentOneofIndex = &idx
			}
			continue
		}

		// Parse fields (both regular and oneof fields)
		if currentMsg != nil && strings.Contains(line, "=") {
			parts := strings.Split(line, "=")
			if len(parts) != 2 {
				continue
			}

			fieldParts := strings.Fields(strings.TrimSpace(parts[0]))
			if len(fieldParts) < 2 {
				// This might be an enum value
				if currentEnum != nil {
					name := strings.TrimSpace(parts[0])
					number := parseFieldNumber(parts[1])
					currentEnum.Value = append(currentEnum.Value, EnumValue{
						Name:   name,
						Number: number,
					})
				}
				continue
			}

			field := Field{
				Type:       fieldParts[0],
				Name:       fieldParts[1],
				Number:     parseFieldNumber(parts[1]),
				OneOfIndex: currentOneofIndex,
			}

			// Handle optional/repeated labels
			if fieldParts[0] == "optional" || fieldParts[0] == "repeated" {
				field.Label = fieldParts[0]
				field.Type = fieldParts[1]
				field.Name = fieldParts[2]
			}

			currentMsg.Field = append(currentMsg.Field, field)
		}

		// Parse enum values
		if currentEnum != nil && strings.Contains(line, "=") {
			parts := strings.Split(line, "=")
			if len(parts) != 2 {
				continue
			}

			// Skip if it's a field declaration (has type)
			if len(strings.Fields(strings.TrimSpace(parts[0]))) > 1 {
				continue
			}

			name := strings.TrimSpace(parts[0])
			number := parseFieldNumber(parts[1])
			currentEnum.Value = append(currentEnum.Value, EnumValue{
				Name:   name,
				Number: number,
			})
		}
	}

	return &desc, nil
}

func countTotalMessages(messages []MessageType) int {
	total := len(messages)
	for _, msg := range messages {
		total += countTotalMessages(msg.NestedType)
	}
	return total
}

func parseFieldNumber(s string) int {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, ";")
	num, _ := strconv.Atoi(s)
	return num
}

func debugPrintDescriptor(desc *Descriptor) {
	bold := color.New(color.Bold)
	blue := color.New(color.FgBlue)
	green := color.New(color.FgGreen)
	yellow := color.New(color.FgYellow)
	purple := color.New(color.FgMagenta)
	cyan := color.New(color.FgCyan)
	red := color.New(color.FgRed)

	for i, msg := range desc.MessageType {
		if i > 0 {
			fmt.Println("----------------------------------------")
		}
		bold.Print(blue.Sprint("> message: "), msg.Name, "\n")

		// Print OneOfs first for context
		if len(msg.OneOfDecl) > 0 {
			purple.Println("  OneOfs:")
			for i, oneof := range msg.OneOfDecl {
				fmt.Printf("    [%s] %s\n",
					yellow.Sprint(i),
					purple.Sprint(oneof.Name),
				)
			}
			fmt.Println()
		}

		// Print fields
		if len(msg.Field) > 0 {
			green.Println("  Fields:")
			for _, field := range msg.Field {
				fieldType := field.Type
				if field.Label != "" {
					fieldType = fmt.Sprintf("%s %s",
						cyan.Sprint(field.Label),
						field.Type,
					)
				}

				var enumValues string
				for _, enum := range msg.EnumType {
					if enum.Name == field.Type {
						values := make([]string, len(enum.Value))
						for i, v := range enum.Value {
							values[i] = fmt.Sprintf("%s=%d", v.Name, v.Number)
						}
						enumValues = fmt.Sprintf(" [%s]", strings.Join(values, ", "))
						break
					}
				}
				if enumValues == "" {
					for _, enum := range desc.EnumType {
						if enum.Name == field.Type {
							values := make([]string, len(enum.Value))
							for i, v := range enum.Value {
								values[i] = fmt.Sprintf("%s=%d", v.Name, v.Number)
							}
							enumValues = fmt.Sprintf(" [%s]", strings.Join(values, ", "))
							break
						}
					}
				}

				if enumValues != "" {
					enumValues = fmt.Sprintf(" %s", enumValues)
				}

				// Print field info
				var fieldStr string
				if field.OneOfIndex != nil && len(msg.OneOfDecl) > *field.OneOfIndex {
					fieldStr = fmt.Sprintf("    %s %s = %s (oneof: %s (%s))%s\n",
						fieldType,
						green.Sprint(field.Name),
						yellow.Sprint(field.Number),
						yellow.Sprint(*field.OneOfIndex),
						purple.Sprint(msg.OneOfDecl[*field.OneOfIndex].Name),
						enumValues,
					)
				} else {
					fieldStr = fmt.Sprintf("    %s %s = %s%s\n",
						fieldType,
						green.Sprint(field.Name),
						yellow.Sprint(field.Number),
						enumValues,
					)
				}
				fmt.Print(fieldStr)
			}
		}

		// Print enums
		if len(msg.EnumType) > 0 {
			red.Println("\n  Enums:")
			for _, enum := range msg.EnumType {
				red.Printf("    %s:\n", enum.Name)
				for _, val := range enum.Value {
					fmt.Printf("      %s = %s\n",
						red.Sprint(val.Name),
						yellow.Sprint(val.Number),
					)
				}
			}
		}
	}
	fmt.Println("----------------------------------------")
}
