package fileutil

import (
	"bufio"
	"fmt"
	"os"
)

// LogReadOptions defines options for reading log files
type LogReadOptions struct {
	Head   int // Number of lines from the beginning
	Tail   int // Number of lines from the end
	Offset int // Line number to start from (1-based)
	Limit  int // Maximum number of lines to return
}

// LogResult represents the result of reading a log file
type LogResult struct {
	Lines      []string // The lines read from the file
	LineCount  int      // Number of lines returned
	TotalLines int      // Total number of lines in the file
	HasMore    bool     // Whether there are more lines available
}

// ReadLogLines reads a specific portion of a log file without loading the entire file into memory
func ReadLogLines(filePath string, options LogReadOptions) (*LogResult, error) {
	// Check if file exists
	if !FileExists(filePath) {
		return nil, fmt.Errorf("file not found: %s", filePath)
	}

	// Get file info for size
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("error getting file info: %w", err)
	}

	// If file is empty, return empty result
	if fileInfo.Size() == 0 {
		return &LogResult{
			Lines:      []string{},
			LineCount:  0,
			TotalLines: 0,
			HasMore:    false,
		}, nil
	}

	// Count total lines in the file
	totalLines, err := countLines(filePath)
	if err != nil {
		return nil, fmt.Errorf("error counting lines: %w", err)
	}

	// If tail is specified, read from the end
	if options.Tail > 0 {
		return readLastLines(filePath, options.Tail, totalLines)
	}

	// If head is specified, read from the beginning
	if options.Head > 0 {
		return readFirstLines(filePath, options.Head, totalLines)
	}

	// If offset and limit are specified, read a specific range
	if options.Offset > 0 {
		limit := options.Limit
		if limit <= 0 {
			limit = 1000 // Default limit
		}
		return readLinesRange(filePath, options.Offset, limit, totalLines)
	}

	// Default: read with a reasonable limit
	limit := options.Limit
	if limit <= 0 {
		limit = 1000 // Default limit
	}
	return readLinesRange(filePath, 1, limit, totalLines)
}

// countLines counts the number of lines in a file
func countLines(filePath string) (int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
	}

	if err := scanner.Err(); err != nil {
		return 0, err
	}

	return lineCount, nil
}

// readFirstLines reads the first n lines from a file
func readFirstLines(filePath string, n int, totalLines int) (*LogResult, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lines := make([]string, 0, n)
	lineCount := 0

	for scanner.Scan() && lineCount < n {
		lines = append(lines, scanner.Text())
		lineCount++
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &LogResult{
		Lines:      lines,
		LineCount:  lineCount,
		TotalLines: totalLines,
		HasMore:    lineCount < totalLines,
	}, nil
}

// readLastLines reads the last n lines from a file
func readLastLines(filePath string, n int, totalLines int) (*LogResult, error) {
	// If n is 0, return empty result
	if n <= 0 {
		return &LogResult{
			Lines:      []string{},
			LineCount:  0,
			TotalLines: totalLines,
			HasMore:    totalLines > 0,
		}, nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// If n is greater than or equal to total lines, read all lines
	if n >= totalLines {
		return readFirstLines(filePath, totalLines, totalLines)
	}

	// Use a ring buffer to keep the last n lines
	ring := make([]string, n)
	scanner := bufio.NewScanner(file)
	lineCount := 0
	ringIndex := 0

	for scanner.Scan() {
		ring[ringIndex] = scanner.Text()
		ringIndex = (ringIndex + 1) % n
		lineCount++
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Rearrange the ring buffer to get the lines in the correct order
	result := make([]string, 0, n)
	if lineCount < n {
		// If we have fewer lines than requested, just return all lines
		result = ring[:lineCount]
	} else {
		// Otherwise, rearrange the ring buffer
		result = append(result, ring[ringIndex:]...)
		result = append(result, ring[:ringIndex]...)
	}

	return &LogResult{
		Lines:      result,
		LineCount:  len(result),
		TotalLines: totalLines,
		HasMore:    len(result) < totalLines,
	}, nil
}

// readLinesRange reads a range of lines from a file
func readLinesRange(filePath string, offset, limit int, totalLines int) (*LogResult, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Adjust offset if it's out of range
	if offset < 1 {
		offset = 1
	}
	if offset > totalLines {
		return &LogResult{
			Lines:      []string{},
			LineCount:  0,
			TotalLines: totalLines,
			HasMore:    false,
		}, nil
	}

	scanner := bufio.NewScanner(file)
	lineNum := 1
	lines := make([]string, 0, limit)

	// Skip lines before the offset
	for lineNum < offset && scanner.Scan() {
		lineNum++
	}

	// Read lines from offset to offset+limit
	for lineNum <= totalLines && len(lines) < limit && scanner.Scan() {
		lines = append(lines, scanner.Text())
		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &LogResult{
		Lines:      lines,
		LineCount:  len(lines),
		TotalLines: totalLines,
		HasMore:    offset+len(lines)-1 < totalLines,
	}, nil
}

// ReadLogContent reads a specific portion of a log file and returns it as a string
func ReadLogContent(filePath string, options LogReadOptions) (string, int, int, bool, error) {
	result, err := ReadLogLines(filePath, options)
	if err != nil {
		return "", 0, 0, false, err
	}

	content := ""
	for i, line := range result.Lines {
		content += line
		if i < len(result.Lines)-1 {
			content += "\n"
		}
	}

	return content, result.LineCount, result.TotalLines, result.HasMore, nil
}
