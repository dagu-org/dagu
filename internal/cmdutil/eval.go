package cmdutil

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
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
func EvalString(input string, opts ...EvalOption) (string, error) {
	options := newEvalOptions()
	for _, opt := range opts {
		opt(options)
	}
	value := input
	for _, vars := range options.Variables {
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
func EvalIntString(input string, opts ...EvalOption) (int, error) {
	options := newEvalOptions()
	for _, opt := range opts {
		opt(options)
	}
	value := input
	for _, vars := range options.Variables {
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

// EvalStringFields processes all string fields in a struct by expanding environment
// variables and substituting command outputs. It takes a struct value and returns a new
// modified struct value.
func EvalStringFields[T any](obj T, opts ...EvalOption) (T, error) {
	options := newEvalOptions()
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

func newEvalOptions() *EvalOptions {
	return &EvalOptions{
		ExpandEnv:  true,
		Substitute: true,
	}
}
