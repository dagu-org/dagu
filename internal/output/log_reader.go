package output

import (
	"bufio"
	"os"
	"strings"
	"unicode"
)

// maxScannerBuffer is the maximum buffer size for reading log lines.
// This allows reading lines up to 1MB in length.
const maxScannerBuffer = 1024 * 1024

// ReadLogFileTail reads the last N lines from a log file.
// It returns:
//   - lines: the actual log lines (tail portion if truncated)
//   - truncated: number of lines that were truncated from the beginning
//   - err: any error encountered while reading
//
// If maxLines is 0 or negative, all lines are returned.
// If the file doesn't exist or is empty, returns nil, 0, nil.
// Binary files are detected and return ["(binary data)"], 0, nil.
func ReadLogFileTail(path string, maxLines int) ([]string, int, error) {
	if path == "" {
		return nil, 0, nil
	}

	// #nosec G304 - file path is from trusted DAG execution status data
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, nil
		}
		return nil, 0, err
	}
	defer func() {
		_ = file.Close()
	}()

	// Read all lines into memory
	var allLines []string
	scanner := bufio.NewScanner(file)

	// Increase buffer size for long lines
	buf := make([]byte, maxScannerBuffer)
	scanner.Buffer(buf, maxScannerBuffer)

	isFirstLine := true
	for scanner.Scan() {
		line := scanner.Text()

		// Check for binary content in first line BEFORE cleaning
		if isFirstLine {
			if isBinaryContent([]byte(line)) {
				return []string{"(binary data)"}, 0, nil
			}
			isFirstLine = false
		}

		// Clean control characters and handle carriage returns (progress bars)
		cleanedLines := cleanLogLine(line)
		allLines = append(allLines, cleanedLines...)
	}

	if err := scanner.Err(); err != nil {
		return nil, 0, err
	}

	// Remove trailing empty lines for cleaner output
	allLines = trimTrailingEmptyLines(allLines)

	// Handle empty file
	if len(allLines) == 0 {
		return nil, 0, nil
	}

	// Return all lines if no limit or fewer lines than limit
	if maxLines <= 0 || len(allLines) <= maxLines {
		return allLines, 0, nil
	}

	// Return last N lines (tail)
	truncated := len(allLines) - maxLines
	return allLines[truncated:], truncated, nil
}

// ReadLogFileAll reads all content from a log file.
// This is a convenience wrapper around ReadLogFileTail with no line limit.
func ReadLogFileAll(path string) ([]string, error) {
	lines, _, err := ReadLogFileTail(path, 0)
	return lines, err
}

// trimTrailingEmptyLines removes empty lines from the end of a slice.
func trimTrailingEmptyLines(lines []string) []string {
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// cleanLogLine processes a log line to handle control characters.
// It handles:
//   - Carriage returns (progress bars): splits into multiple lines, keeping last segment
//   - Other control characters: removed
//   - Trailing whitespace: trimmed
func cleanLogLine(line string) []string {
	// Handle carriage returns (common in progress bars like curl)
	// Only keep the final segment after the last \r
	if strings.Contains(line, "\r") {
		segments := strings.Split(line, "\r")
		// Get the last non-empty segment (the final state of the line)
		for i := len(segments) - 1; i >= 0; i-- {
			segment := strings.TrimRightFunc(segments[i], unicode.IsSpace)
			if segment != "" {
				return []string{cleanControlChars(segment)}
			}
		}
		return nil
	}

	// Clean other control characters and trim
	cleaned := cleanControlChars(line)
	cleaned = strings.TrimRightFunc(cleaned, unicode.IsSpace)
	if cleaned == "" {
		return nil
	}
	return []string{cleaned}
}

// cleanControlChars removes non-printable control characters from a string,
// preserving tabs and spaces.
func cleanControlChars(s string) string {
	var result strings.Builder
	result.Grow(len(s))

	for _, r := range s {
		// Keep printable characters, tabs, and spaces
		if r >= 32 || r == '\t' {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// isBinaryContent checks if the content appears to be binary data.
// It uses two heuristics:
//  1. Presence of null bytes (strong indicator)
//  2. High percentage (>30%) of non-printable characters
func isBinaryContent(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	// Check for null bytes (strong indicator of binary)
	for _, b := range data {
		if b == 0 {
			return true
		}
	}

	// Count non-printable characters (excluding common whitespace)
	nonPrintable := 0
	for _, b := range data {
		if b < 32 && b != '\t' && b != '\n' && b != '\r' {
			nonPrintable++
		}
	}

	// If more than 30% non-printable, consider it binary
	return float64(nonPrintable)/float64(len(data)) > 0.3
}
