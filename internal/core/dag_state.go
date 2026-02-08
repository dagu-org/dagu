package core

import "time"

// DAGState holds scheduler-managed per-DAG state.
type DAGState struct {
	LastTick time.Time
}
