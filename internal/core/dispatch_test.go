package core

import (
	"testing"

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/stretchr/testify/assert"
)

func TestShouldDispatchToCoordinator(t *testing.T) {
	tests := []struct {
		name           string
		dag            *DAG
		hasCoordinator bool
		defaultMode    config.ExecutionMode
		want           bool
	}{
		{
			name:           "ForceLocal is true — always local",
			dag:            &DAG{ForceLocal: true, WorkerSelector: map[string]string{"gpu": "true"}},
			hasCoordinator: true,
			defaultMode:    config.ExecutionModeDistributed,
			want:           false,
		},
		{
			name:           "no coordinator — always local",
			dag:            &DAG{WorkerSelector: map[string]string{"gpu": "true"}},
			hasCoordinator: false,
			defaultMode:    config.ExecutionModeDistributed,
			want:           false,
		},
		{
			name:           "workerSelector present — dispatch",
			dag:            &DAG{WorkerSelector: map[string]string{"gpu": "true"}},
			hasCoordinator: true,
			defaultMode:    config.ExecutionModeLocal,
			want:           true,
		},
		{
			name:           "defaultMode distributed — dispatch",
			dag:            &DAG{},
			hasCoordinator: true,
			defaultMode:    config.ExecutionModeDistributed,
			want:           true,
		},
		{
			name:           "defaultMode local, no workerSelector — local",
			dag:            &DAG{},
			hasCoordinator: true,
			defaultMode:    config.ExecutionModeLocal,
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldDispatchToCoordinator(tt.dag, tt.hasCoordinator, tt.defaultMode)
			assert.Equal(t, tt.want, got)
		})
	}
}
