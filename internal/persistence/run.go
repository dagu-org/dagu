package persistence

// Run represents metadata about a DAG run
type Run struct {
	File   string
	Status Status
}

// NewRun creates a new Run instance with the specified file path and status
func NewRun(file string, status Status) *Run {
	return &Run{
		File:   file,
		Status: status,
	}
}
