package sql

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// namedParamRegex matches named parameters like :param_name or :paramName
var namedParamRegex = regexp.MustCompile(`:([a-zA-Z_][a-zA-Z0-9_]*)`)

// positionalParamRegex matches positional parameters like $1, $2, etc.
var positionalParamRegex = regexp.MustCompile(`\$(\d+)`)

// ConvertNamedToPositional converts named parameters (:param) to positional parameters ($1, $2, ...).
// Returns the converted query and ordered parameter values.
func ConvertNamedToPositional(query string, params map[string]any, placeholder string) (string, []any, error) {
	if params == nil {
		return query, nil, nil
	}

	// Find all named parameters in the query
	matches := namedParamRegex.FindAllStringSubmatchIndex(query, -1)
	if len(matches) == 0 {
		return query, nil, nil
	}

	// Track parameter positions for deduplication
	paramPositions := make(map[string]int)
	var orderedParams []any
	var result strings.Builder

	lastEnd := 0
	for _, match := range matches {
		// match[0:2] is the full match, match[2:4] is the capture group
		paramStart := match[0]
		paramEnd := match[1]
		nameStart := match[2]
		nameEnd := match[3]

		paramName := query[nameStart:nameEnd]

		// Check if parameter exists in params map
		value, ok := params[paramName]
		if !ok {
			return "", nil, fmt.Errorf("parameter %q not found in params", paramName)
		}

		// Write text before this parameter
		result.WriteString(query[lastEnd:paramStart])

		// Get or assign position for this parameter
		pos, exists := paramPositions[paramName]
		if !exists {
			orderedParams = append(orderedParams, value)
			pos = len(orderedParams)
			paramPositions[paramName] = pos
		}

		// Write the positional placeholder
		if placeholder == "?" {
			// For ? placeholder (SQLite), each occurrence needs its own parameter value
			result.WriteString("?")
			if exists {
				// Duplicate the value for repeated parameters
				orderedParams = append(orderedParams, value)
			}
		} else {
			// For $N placeholder (PostgreSQL), reuse the same position
			result.WriteString(placeholder + strconv.Itoa(pos))
		}

		lastEnd = paramEnd
	}

	// Write remaining text after last parameter
	result.WriteString(query[lastEnd:])

	return result.String(), orderedParams, nil
}

// ConvertPositionalParams validates and returns positional parameters.
// Ensures the query has the correct number of placeholders.
func ConvertPositionalParams(query string, params []any, placeholder string) ([]any, error) {
	if params == nil {
		return nil, nil
	}

	// Count placeholders in query
	var count int
	if placeholder == "?" {
		count = strings.Count(query, "?")
	} else {
		// Count $N placeholders using regex to find the highest numbered placeholder
		matches := positionalParamRegex.FindAllStringSubmatch(query, -1)
		for _, match := range matches {
			if len(match) > 1 {
				n, err := strconv.Atoi(match[1])
				if err == nil && n > count {
					count = n
				}
			}
		}
	}

	if count != len(params) {
		return nil, fmt.Errorf("parameter count mismatch: query has %d placeholders but %d parameters provided", count, len(params))
	}

	return params, nil
}

// PrepareParams prepares parameters for query execution.
// Handles both named and positional parameters.
func PrepareParams(query string, cfg *Config, driver Driver) (string, []any, error) {
	// Check for named parameters
	if namedParams, ok := cfg.GetNamedParams(); ok {
		return driver.ConvertNamedParams(query, namedParams)
	}

	// Check for positional parameters
	if positionalParams, ok := cfg.GetPositionalParams(); ok {
		params, err := ConvertPositionalParams(query, positionalParams, driver.PlaceholderFormat())
		return query, params, err
	}

	// No parameters
	return query, nil, nil
}

// SanitizeIdentifier sanitizes a SQL identifier (table/column name) to prevent injection.
// Only allows alphanumeric characters, underscores, and dots (for schema.table notation).
func SanitizeIdentifier(identifier string) (string, error) {
	// Check for empty string
	if identifier == "" {
		return "", fmt.Errorf("identifier cannot be empty")
	}

	// Validate characters
	for i, r := range identifier {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') &&
			!(r >= '0' && r <= '9') && r != '_' && r != '.' {
			return "", fmt.Errorf("invalid character %q at position %d in identifier %q", r, i, identifier)
		}
	}

	// First character cannot be a digit
	if identifier[0] >= '0' && identifier[0] <= '9' {
		return "", fmt.Errorf("identifier %q cannot start with a digit", identifier)
	}

	return identifier, nil
}

// ExtractParamNames extracts parameter names from a query with named parameters.
// Returns the names in the order they appear.
func ExtractParamNames(query string) []string {
	matches := namedParamRegex.FindAllStringSubmatch(query, -1)
	seen := make(map[string]bool)
	var names []string

	for _, match := range matches {
		if len(match) > 1 {
			name := match[1]
			if !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}

	return names
}

// ValidateParams checks if all required parameters are provided.
func ValidateParams(query string, params map[string]any) error {
	required := ExtractParamNames(query)
	var missing []string

	for _, name := range required {
		if _, ok := params[name]; !ok {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("missing required parameters: %s", strings.Join(missing, ", "))
	}

	return nil
}
