package cmdutil

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/dagu-org/dagu/internal/logger"
	"github.com/itchyny/gojq"
)

type EvalOptions struct {
	ExpandEnv  bool
	Substitute bool
	Variables  []map[string]string
}

type EvalOption func(*EvalOptions)

func WithVariables(vars map[string]string) EvalOption {
	return func(opts *EvalOptions) {
		opts.Variables = append(opts.Variables, vars)
	}
}

func WithoutExpandEnv() EvalOption {
	return func(opts *EvalOptions) {
		opts.ExpandEnv = false
	}
}

func WithoutSubstitute() EvalOption {
	return func(opts *EvalOptions) {
		opts.Substitute = false
	}
}

func OnlyReplaceVars() EvalOption {
	return func(opts *EvalOptions) {
		opts.ExpandEnv = false
		opts.Substitute = false
	}
}

var reEscapedKeyValue = regexp.MustCompile(`^[^\s=]+="[^"]+"$`)

// BuildCommandEscapedString constructs a single shell-ready string from a command and its arguments.
// It assumes that the command and arguments are already escaped.
func BuildCommandEscapedString(command string, args []string) string {
	quotedArgs := make([]string, 0, len(args))
	for _, arg := range args {
		// If already quoted, skip
		if strings.HasPrefix(arg, `"`) && strings.HasSuffix(arg, `"`) {
			quotedArgs = append(quotedArgs, arg)
			continue
		}
		if strings.HasPrefix(arg, `'`) && strings.HasSuffix(arg, `'`) {
			quotedArgs = append(quotedArgs, arg)
			continue
		}
		// If the argument contains spaces, quote it.
		if strings.ContainsAny(arg, " ") {
			// If it includes '=' and is already quoted, skip
			if reEscapedKeyValue.MatchString(arg) {
				quotedArgs = append(quotedArgs, arg)
				continue
			}
			// if it contains double quotes, escape them
			arg = strings.ReplaceAll(arg, `"`, `\"`)
			quotedArgs = append(quotedArgs, fmt.Sprintf(`"%s"`, arg))
		} else {
			quotedArgs = append(quotedArgs, arg)
		}
	}

	// If we have no arguments, just return the command without trailing space.
	if len(quotedArgs) == 0 {
		return command
	}

	return fmt.Sprintf("%s %s", command, strings.Join(quotedArgs, " "))
}

// EvalString substitutes environment variables and commands in the input string
func EvalString(ctx context.Context, input string, opts ...EvalOption) (string, error) {
	if input == "" {
		return "", nil // nothing to do
	}

	options := newEvalOptions()
	for _, opt := range opts {
		opt(options)
	}
	value := input
	for _, vars := range options.Variables {
		value = ExpandReferences(ctx, value, vars)
		value = replaceVars(value, vars)
	}
	if options.Substitute {
		var err error
		value, err = substituteCommands(value)
		if err != nil {
			return "", fmt.Errorf("failed to substitute string in %q: %w", input, err)
		}
	}
	if options.ExpandEnv {
		value = os.ExpandEnv(value)
	}
	return value, nil
}

// EvalIntString substitutes environment variables and commands in the input string
func EvalIntString(ctx context.Context, input string, opts ...EvalOption) (int, error) {
	options := newEvalOptions()
	for _, opt := range opts {
		opt(options)
	}
	value := input
	for _, vars := range options.Variables {
		value = ExpandReferences(ctx, value, vars)
		value = replaceVars(value, vars)
	}
	if options.ExpandEnv {
		value = os.ExpandEnv(value)
	}
	value, err := substituteCommands(value)
	if err != nil {
		return 0, err
	}
	v, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("failed to convert %q to int: %w", value, err)
	}
	return v, nil
}

// EvalStringFields processes all string fields in a struct or map by expanding environment
// variables and substituting command outputs. It takes a struct or map value and returns a new
// modified struct or map value.
func EvalStringFields[T any](ctx context.Context, obj T, opts ...EvalOption) (T, error) {
	options := newEvalOptions()
	for _, opt := range opts {
		opt(options)
	}

	v := reflect.ValueOf(obj)

	// Handle different types
	// nolint:exhaustive
	switch v.Kind() {
	case reflect.Struct:
		modified := reflect.New(v.Type()).Elem()
		modified.Set(v)

		if err := processStructFields(ctx, modified, options); err != nil {
			return obj, fmt.Errorf("failed to process struct fields: %w", err)
		}

		return modified.Interface().(T), nil

	case reflect.Map:
		result, err := processMap(ctx, v, options)
		if err != nil {
			return obj, fmt.Errorf("failed to process map: %w", err)
		}
		return result.Interface().(T), nil

	default:
		return obj, fmt.Errorf("input must be a struct or map, got %T", obj)
	}
}

func processStructFields(ctx context.Context, v reflect.Value, opts *EvalOptions) error {
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if !field.CanSet() {
			continue
		}

		// nolint:exhaustive
		switch field.Kind() {
		case reflect.String:
			value := field.String()
			for _, vars := range opts.Variables {
				value = replaceVars(value, vars)
			}

			if opts.Substitute {
				var err error
				value, err = substituteCommands(value)
				if err != nil {
					return fmt.Errorf("field %q: %w", t.Field(i).Name, err)
				}
			}

			if opts.ExpandEnv {
				value = os.ExpandEnv(value)
			}

			field.SetString(value)

		case reflect.Struct:
			if err := processStructFields(ctx, field, opts); err != nil {
				return err
			}

		case reflect.Map:
			// Process map fields
			if field.IsNil() {
				continue
			}

			processed, err := processMap(ctx, field, opts)
			if err != nil {
				return fmt.Errorf("field %q: %w", t.Field(i).Name, err)
			}

			field.Set(processed)
		}
	}
	return nil
}

// processMap recursively processes a map, evaluating string values and recursively processing
// nested maps and structs.
func processMap(ctx context.Context, v reflect.Value, opts *EvalOptions) (reflect.Value, error) {
	// Create a new map of the same type
	mapType := v.Type()
	newMap := reflect.MakeMap(mapType)

	// Iterate over the map entries
	iter := v.MapRange()
	for iter.Next() {
		key := iter.Key()
		val := iter.Value()

		// Process the value based on its type
		var newVal reflect.Value
		var err error

		for (val.Kind() == reflect.Interface || val.Kind() == reflect.Ptr) && !val.IsNil() {
			val = val.Elem()
		}

		// nolint:exhaustive
		switch val.Kind() {
		case reflect.String:
			// Evaluate string values
			strVal := val.String()
			for _, vars := range opts.Variables {
				strVal = replaceVars(strVal, vars)
			}

			if opts.Substitute {
				var err error
				strVal, err = substituteCommands(strVal)
				if err != nil {
					return v, fmt.Errorf("map value: %w", err)
				}
			}

			if opts.ExpandEnv {
				strVal = os.ExpandEnv(strVal)
			}

			newVal = reflect.ValueOf(strVal)

		case reflect.Map:
			// Recursively process nested maps
			newVal, err = processMap(ctx, val, opts)
			if err != nil {
				return v, err
			}

		case reflect.Struct:
			// Process structs
			structCopy := reflect.New(val.Type()).Elem()
			structCopy.Set(val)

			if err := processStructFields(ctx, structCopy, opts); err != nil {
				return v, err
			}

			newVal = structCopy

		default:
			// Keep other types as is
			newVal = val
		}

		// Set the new value in the map
		newMap.SetMapIndex(key, newVal)
	}

	return newMap, nil
}

// ExpandReferences finds all occurrences of ${NAME.foo.bar} in the input string,
// where "NAME" matches a key in the dataMap. The dataMap value is expected to be
// JSON. It then uses gojq to extract the .foo.bar sub-path from that JSON
// document. If successful, it replaces the original placeholder with the sub-path value.
//
// If dataMap[name] is invalid JSON or the sub-path does not exist,
// the placeholder is left as-is (or you could handle it differently).
func ExpandReferences(ctx context.Context, input string, dataMap map[string]string) string {
	// Regex to match patterns like ${FOO.bar.baz}, capturing:
	//   group 1 => FOO  (the top-level name)
	//   group 2 => .bar.baz (the path portion)
	// Explanation:
	//   \${            matches literal ${
	//   ([A-Za-z0-9_]\w*) captures a variable name starting with letter/underscore
	//   (              start capture for the path
	//     \.[^}]+      match a '.' then anything up to a '}', allowing dot notation
	//   )              end capture group for path
	//   }              matches literal }
	//
	re := regexp.MustCompile(`\$\{([A-Za-z0-9_]\w*)(\.[^}]+)\}|\$([A-Za-z0-9_]\w*)(\.[^\s]+)`)

	// We'll do a "ReplaceAllStringFunc" approach. For each match, we parse out the JSON path.
	result := re.ReplaceAllStringFunc(input, func(match string) string {
		subMatches := re.FindStringSubmatch(match)
		if len(subMatches) < 3 {
			// Shouldn't happen given the pattern, but just in case:
			return match
		}

		var name string
		var path string
		if strings.HasPrefix(subMatches[0], "${") {
			name = subMatches[1] // e.g. "FOO"
			path = subMatches[2] // e.g. ".bar.baz"
		} else {
			name = subMatches[3] // e.g. "FOO"
			path = subMatches[4] // e.g. ".bar.baz"
		}

		// Lookup the JSON content for this "name"
		jsonStr, ok := dataMap[name]
		if !ok {
			// Find the variable from the environment
			val, ok := os.LookupEnv(name)
			if !ok {
				// Not found => leave as-is or handle otherwise
				return match
			}
			jsonStr = val
		}

		// Try to parse it as JSON and evaluate path
		var raw any
		if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
			// Not valid JSON => leave as-is
			return match
		}

		// Build a gojq query (like .bar.baz)
		query, err := gojq.Parse(path)
		if err != nil {
			// If parsing the path fails => leave as-is
			logger.Warn(ctx, "failed to parse path %q in data %q: %v", path, name, err)
			return match
		}

		// Run the query
		iter := query.Run(raw)
		v, ok := iter.Next()
		if !ok {
			// Means no result => leave placeholder as-is
			logger.Warn(ctx, "no result for path %q in data %q", path, name)
			return match
		}

		// If gojq yields an error or multiple results, handle that:
		if _, isErr := v.(error); isErr {
			// Some query error => leave as-is
			logger.Warn(ctx, "error evaluating path %q in data %q: %v", path, name, v)
			return match
		}

		// v is the sub-path's value => convert to string
		// If it's a map/array, you might want to re-marshal to JSON, but let's do a simple fmt
		replacement := fmt.Sprintf("%v", v)
		return replacement
	})

	return result
}

func replaceVars(template string, vars map[string]string) string {
	re := regexp.MustCompile(`[']{0,1}\$\{([^}]+)\}[']{0,1}|[']{0,1}\$([a-zA-Z0-9_][a-zA-Z0-9_]*)[']{0,1}`)

	return re.ReplaceAllStringFunc(template, func(match string) string {
		var key string
		if match[0] == '\'' && match[len(match)-1] == '\'' {
			// If the match is surrounded by single quotes, leave it as-is
			return match
		}
		if strings.HasPrefix(match, "${") {
			key = match[2 : len(match)-1]
		} else {
			key = match[1:]
		}

		if val, ok := vars[key]; ok {
			return val
		}
		return match
	})
}

func newEvalOptions() *EvalOptions {
	return &EvalOptions{
		ExpandEnv:  true,
		Substitute: true,
	}
}
