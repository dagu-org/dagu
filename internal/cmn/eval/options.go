package eval

// Options controls the behavior of string evaluation.
type Options struct {
	ExpandEnv   bool
	Substitute  bool
	Variables   []map[string]string
	StepMap     map[string]StepInfo
	ExpandShell bool // When false, skip shell-based variable expansion (e.g., for SSH commands)
}

// NewOptions returns default Options with all features enabled.
func NewOptions() *Options {
	return &Options{
		ExpandEnv:   true,
		ExpandShell: true,
		Substitute:  true,
	}
}

// Option is a functional option for configuring evaluation.
type Option func(*Options)

// WithVariables adds a variable map for expansion.
func WithVariables(vars map[string]string) Option {
	return func(opts *Options) {
		opts.Variables = append(opts.Variables, vars)
	}
}

// WithStepMap sets the step info map for step reference expansion.
func WithStepMap(stepMap map[string]StepInfo) Option {
	return func(opts *Options) {
		opts.StepMap = stepMap
	}
}

// WithoutExpandEnv disables environment variable expansion.
func WithoutExpandEnv() Option {
	return func(opts *Options) {
		opts.ExpandEnv = false
	}
}

// WithoutExpandShell disables shell-based variable expansion.
func WithoutExpandShell() Option {
	return func(opts *Options) {
		opts.ExpandShell = false
	}
}

// WithoutSubstitute disables backtick command substitution.
func WithoutSubstitute() Option {
	return func(opts *Options) {
		opts.Substitute = false
	}
}

// OnlyReplaceVars disables both env expansion and command substitution,
// leaving only explicit variable replacement.
func OnlyReplaceVars() Option {
	return func(opts *Options) {
		opts.ExpandEnv = false
		opts.Substitute = false
	}
}
