package digraph

// ParallelConfig contains the configuration for parallel execution of a step.
// MVP version supports basic parallel execution with maxConcurrent control.
type ParallelConfig struct {
	// Items is the array of items to process in parallel.
	// Can be a direct array or a reference to a variable containing an array.
	Items []any `json:"items,omitempty"`
	
	// MaxConcurrent is the maximum number of parallel executions.
	// Default is 10 if not specified.
	MaxConcurrent int `json:"maxConcurrent,omitempty"`
}

// DefaultMaxConcurrent is the default maximum concurrent executions for parallel steps
const DefaultMaxConcurrent = 10

// ParallelItemVariable is the special variable name that represents the current item in parallel execution
const ParallelItemVariable = "ITEM"