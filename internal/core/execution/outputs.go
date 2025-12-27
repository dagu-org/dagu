package execution

// DAGRunOutputs represents the full outputs file structure with metadata.
// This is version 2 of the outputs schema, which includes execution context.
type DAGRunOutputs struct {
	Version  int               `json:"version"`
	Metadata OutputsMetadata   `json:"metadata"`
	Outputs  map[string]string `json:"outputs"`
}

// OutputsMetadata contains execution context for the outputs.
type OutputsMetadata struct {
	DAGName     string `json:"dagName"`
	DAGRunID    string `json:"dagRunId"`
	AttemptID   string `json:"attemptId"`
	Status      string `json:"status"`
	CompletedAt string `json:"completedAt"`
	Params      string `json:"params,omitempty"`
}

// OutputsSchemaVersion is the current schema version for outputs.json
const OutputsSchemaVersion = 2
