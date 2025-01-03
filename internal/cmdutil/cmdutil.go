package cmdutil

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/mattn/go-shellwords"
)

var ErrCommandIsEmpty = fmt.Errorf("command is empty")

// ParsePipedCommand splits a shell-style command string into a pipeline ([][]string).
// Each sub-slice represents a single command. Unquoted "|" tokens define the boundaries.
//
// Example:
//
//	parsePipedCommand(`echo foo | grep foo | wc -l`) =>
//	  [][]string{
//	    {"echo", "foo"},
//	    {"grep", "foo"},
//	    {"wc", "-l"},
//	  }
//
//	parsePipedCommand(`echo "hello|world"`) =>
//	  [][]string{ {"echo", "hello|world"} } // single command
func ParsePipedCommand(cmdString string) ([][]string, error) {
	var inQuote, inBacktick, inEscape bool
	var current []rune
	var tokens []string

	for _, r := range cmdString {
		switch {
		case inEscape:
			current = append(current, r)
			inEscape = false
		case r == '\\':
			current = append(current, r)
			inEscape = true
		case r == '"' && !inBacktick:
			current = append(current, r)
			inQuote = !inQuote
		case r == '`':
			current = append(current, r)
			inBacktick = !inBacktick
		case r == '|' && !inQuote && !inBacktick:
			if len(current) > 0 {
				tokens = append(tokens, string(current))
				current = nil
			}
			tokens = append(tokens, "|")
		case unicode.IsSpace(r) && !inQuote && !inBacktick:
			if len(current) > 0 {
				tokens = append(tokens, string(current))
				current = nil
			}
		default:
			current = append(current, r)
		}
	}

	if len(current) > 0 {
		tokens = append(tokens, string(current))
	}

	var pipeline [][]string
	var currentCmd []string

	for _, token := range tokens {
		if token == "|" {
			if len(currentCmd) > 0 {
				pipeline = append(pipeline, currentCmd)
				currentCmd = nil
			}
		} else {
			currentCmd = append(currentCmd, token)
		}
	}

	if len(currentCmd) > 0 {
		pipeline = append(pipeline, currentCmd)
	}

	return pipeline, nil
}

func SplitCommandWithEval(cmd string) (string, []string, error) {
	pipeline, err := ParsePipedCommand(cmd)
	if err != nil {
		return "", nil, err
	}

	parser := shellwords.NewParser()
	parser.ParseBacktick = true
	parser.ParseEnv = false

	for _, command := range pipeline {
		if len(command) < 2 {
			continue
		}
		for i, arg := range command {
			command[i] = arg
			// Escape the command
			command[i] = escapeReplacer.Replace(command[i])
			// Substitute command in the command.
			command[i], err = SubstituteCommands(command[i])
			if err != nil {
				return "", nil, fmt.Errorf("failed to substitute command: %w", err)
			}
		}
	}

	if len(pipeline) > 1 {
		first := pipeline[0]
		cmd := first[0]
		args := first[1:]
		for _, command := range pipeline[1:] {
			args = append(args, "|")
			args = append(args, command...)
		}
		return cmd, args, nil
	}

	if len(pipeline) == 0 {
		return "", nil, ErrCommandIsEmpty
	}

	command := pipeline[0]
	if len(command) == 0 {
		return "", nil, ErrCommandIsEmpty
	}

	return command[0], command[1:], nil
}

var (
	escapeReplacer = strings.NewReplacer(
		`\t`, `\\\\t`,
		`\r`, `\\\\r`,
		`\n`, `\\\\n`,
	)
)

func SplitCommand(cmd string) (string, []string, error) {
	pipeline, err := ParsePipedCommand(cmd)
	if err != nil {
		return "", nil, err
	}

	if len(pipeline) > 1 {
		first := pipeline[0]
		cmd := first[0]
		args := first[1:]
		for _, command := range pipeline[1:] {
			args = append(args, "|")
			args = append(args, command...)
		}
		return cmd, args, nil
	}

	if len(pipeline) == 0 {
		return "", nil, ErrCommandIsEmpty
	}

	command := pipeline[0]
	if len(command) == 0 {
		return "", nil, ErrCommandIsEmpty
	}

	return command[0], command[1:], nil
}

// tickerMatcher matches the command in the value string.
// Example: "`date`"
var tickerMatcher = regexp.MustCompile("`[^`]+`")

// SubstituteCommands substitutes command in the value string.
// This logic needs to be refactored to handle more complex cases.
func SubstituteCommands(input string) (string, error) {
	matches := tickerMatcher.FindAllString(strings.TrimSpace(input), -1)
	if matches == nil {
		return input, nil
	}

	ret := input
	for i := 0; i < len(matches); i++ {
		// Execute the command and replace the command with the output.
		command := matches[i]

		parser := shellwords.NewParser()
		parser.ParseBacktick = true
		parser.ParseEnv = false

		res, err := parser.Parse(escapeReplacer.Replace(command))
		if err != nil {
			return "", fmt.Errorf("failed to substitute command: %w", err)
		}

		ret = strings.ReplaceAll(ret, command, strings.Join(res, " "))
	}

	return ret, nil
}

// GetShellCommand returns the shell to use for command execution
func GetShellCommand(configuredShell string) string {
	if configuredShell != "" {
		return configuredShell
	}

	// Try system shell first
	if systemShell := os.ExpandEnv("${SHELL}"); systemShell != "" {
		return systemShell
	}

	// Fallback to sh if available
	if shPath, err := exec.LookPath("sh"); err == nil {
		return shPath
	}

	return ""
}

type EvalOptions struct {
	Variables []map[string]string
}

type EvalOption func(*EvalOptions)

func WithVariables(vars map[string]string) EvalOption {
	return func(opts *EvalOptions) {
		opts.Variables = append(opts.Variables, vars)
	}
}

// RunExitCode runs the command and returns the exit code.
// func RunExitCode(cmd string) (int, error) {
// 	command, args, err := SplitCommandWithEval(cmd)
// 	if err != nil {
// 		return 0, err
// 	}
// 	shellCommand := GetShellCommand("")
// }

var regEscapedKeyValue = regexp.MustCompile(`^[^\s=]+="[^"]+"$`)

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
			if regEscapedKeyValue.MatchString(arg) {
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
func EvalString(input string, opts ...EvalOption) (string, error) {
	options := &EvalOptions{}
	for _, opt := range opts {
		opt(options)
	}
	value := input
	for _, vars := range options.Variables {
		value = replaceVars(value, vars)
	}
	value = os.ExpandEnv(value)
	value, err := SubstituteCommands(value)
	if err != nil {
		return "", fmt.Errorf("failed to substitute string in %q: %w", input, err)
	}
	return value, nil
}

// EvalIntString substitutes environment variables and commands in the input string
func EvalIntString(input string, opts ...EvalOption) (int, error) {
	options := &EvalOptions{}
	for _, opt := range opts {
		opt(options)
	}
	value := input
	for _, vars := range options.Variables {
		value = replaceVars(value, vars)
	}
	value = os.ExpandEnv(value)
	value, err := SubstituteCommands(value)
	if err != nil {
		return 0, err
	}
	v, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("failed to convert %q to int: %w", value, err)
	}
	return v, nil
}

// EvalStringFields processes all string fields in a struct by expanding environment
// variables and substituting command outputs. It takes a struct value and returns a new
// modified struct value.
func EvalStringFields[T any](obj T, opts ...EvalOption) (T, error) {
	options := &EvalOptions{}
	for _, opt := range opts {
		opt(options)
	}

	v := reflect.ValueOf(obj)
	if v.Kind() != reflect.Struct {
		return obj, fmt.Errorf("input must be a struct, got %T", obj)
	}

	modified := reflect.New(v.Type()).Elem()
	modified.Set(v)

	if err := processStructFields(modified, options); err != nil {
		return obj, fmt.Errorf("failed to process fields: %w", err)
	}

	return modified.Interface().(T), nil
}

func processStructFields(v reflect.Value, opts *EvalOptions) error {
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
			value = os.ExpandEnv(value)
			processed, err := SubstituteCommands(value)
			if err != nil {
				return fmt.Errorf("field %q: %w", t.Field(i).Name, err)
			}
			field.SetString(processed)

		case reflect.Struct:
			if err := processStructFields(field, opts); err != nil {
				return err
			}
		}
	}
	return nil
}

func replaceVars(template string, vars map[string]string) string {
	re := regexp.MustCompile(`\$\{([^}]+)\}|\$([a-zA-Z_][a-zA-Z0-9_]*)`)

	return re.ReplaceAllStringFunc(template, func(match string) string {
		var key string
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
