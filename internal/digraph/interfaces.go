// Copyright (C) 2025 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package digraph

import "context"

// DBClient gets a result of a DAG execution.
type DBClient interface {
	GetDAG(ctx context.Context, name string) (*DAG, error)
	GetStatus(ctx context.Context, name string, requestID string) (*Status, error)
}

// Status is the result of a DAG execution.
type Status struct {
	// Name represents the name of the executed DAG.
	Name string `json:"name,omitempty"`
	// Params is the parameters of the DAG execution
	Params string `json:"params,omitempty"`
	// Outputs is the outputs of the DAG execution.
	Outputs map[string]string `json:"outputs,omitempty"`
}
