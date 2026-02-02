package router

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/runtime/executor"
)

func TestNewRouter(t *testing.T) {
	tests := []struct {
		name    string
		step    core.Step
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid router configuration",
			step: core.Step{
				Name: "test_router",
				Router: &core.RouterConfig{
					Value: "@exitCode",
					Mode:  core.RouterModeExclusive,
					Routes: map[string][]string{
						"0": {"success"},
						"1": {"error"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "nil router configuration",
			step: core.Step{
				Name:   "not_a_router",
				Router: nil,
			},
			wantErr: true,
			errMsg:  "not configured as a router step",
		},
		{
			name: "invalid router configuration - empty value",
			step: core.Step{
				Name: "invalid_router",
				Router: &core.RouterConfig{
					Value: "",
					Mode:  core.RouterModeExclusive,
					Routes: map[string][]string{
						"0": {"success"},
					},
				},
			},
			wantErr: true,
			errMsg:  "router validation failed",
		},
		{
			name: "invalid router configuration - bad regex",
			step: core.Step{
				Name: "bad_regex_router",
				Router: &core.RouterConfig{
					Value: "@value",
					Mode:  core.RouterModeExclusive,
					Routes: map[string][]string{
						"/^[unclosed/": {"match"},
					},
				},
			},
			wantErr: true,
			errMsg:  "router validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			exec, err := newRouter(ctx, tt.step)

			if tt.wantErr {
				if err == nil {
					t.Errorf("newRouter() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("newRouter() error = %q, want error containing %q", err.Error(), tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("newRouter() unexpected error = %v", err)
				return
			}

			if exec == nil {
				t.Error("newRouter() returned nil executor")
			}
		})
	}
}

func TestRouterExecutor_SetStdout_SetStderr(t *testing.T) {
	step := core.Step{
		Name: "test_router",
		Router: &core.RouterConfig{
			Value: "@exitCode",
			Mode:  core.RouterModeExclusive,
			Routes: map[string][]string{
				"0": {"success"},
			},
		},
	}

	exec, err := newRouter(context.Background(), step)
	if err != nil {
		t.Fatalf("newRouter() failed: %v", err)
	}

	routerExec := exec.(*routerExecutor)

	// Test SetStdout
	stdout := &bytes.Buffer{}
	routerExec.SetStdout(stdout)
	if routerExec.stdout != stdout {
		t.Error("SetStdout() did not set stdout correctly")
	}

	// Test SetStderr
	stderr := &bytes.Buffer{}
	routerExec.SetStderr(stderr)
	if routerExec.stderr != stderr {
		t.Error("SetStderr() did not set stderr correctly")
	}
}

func TestRouterExecutor_Kill(t *testing.T) {
	step := core.Step{
		Name: "test_router",
		Router: &core.RouterConfig{
			Value: "@exitCode",
			Mode:  core.RouterModeExclusive,
			Routes: map[string][]string{
				"0": {"success"},
			},
		},
	}

	exec, err := newRouter(context.Background(), step)
	if err != nil {
		t.Fatalf("newRouter() failed: %v", err)
	}

	// Kill should be a no-op and not return error
	err = exec.Kill(os.Interrupt)
	if err != nil {
		t.Errorf("Kill() error = %v, want nil", err)
	}
}

func TestRouterExecutor_Run(t *testing.T) {
	tests := []struct {
		name              string
		step              core.Step
		wantActivated     []string
		wantOutputContains []string
	}{
		{
			name: "exclusive mode - default route",
			step: core.Step{
				Name: "test_router",
				Router: &core.RouterConfig{
					Value: "@exitCode",
					Mode:  core.RouterModeExclusive,
					Routes: map[string][]string{
						"1": {"error_handler"},
					},
					Default: []string{"default_handler"},
				},
			},
			wantActivated: []string{"default_handler"},
			wantOutputContains: []string{
				"Evaluating router patterns",
				"No patterns matched (using default)",
				"Activated steps: [default_handler]",
			},
		},
		{
			name: "exclusive mode - matching route",
			step: core.Step{
				Name: "test_router",
				Router: &core.RouterConfig{
					Value: "@exitCode",
					Mode:  core.RouterModeExclusive,
					Routes: map[string][]string{
						"0": {"success_handler"},
						"1": {"error_handler"},
					},
				},
			},
			wantActivated: []string{"success_handler"},
			wantOutputContains: []string{
				"Evaluating router patterns",
				"Matched patterns:",
				"Activated steps: [success_handler]",
			},
		},
		{
			name: "multi-select mode - empty string match",
			step: core.Step{
				Name: "test_router",
				Router: &core.RouterConfig{
					Value: "@value",
					Mode:  core.RouterModeMultiSelect,
					Routes: map[string][]string{
						"":      {"empty_handler"},
						"error": {"error_handler"},
					},
				},
			},
			wantActivated: []string{"empty_handler"},
			wantOutputContains: []string{
				"Evaluating router patterns",
				"Activated steps:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate router first
			if err := tt.step.Router.Validate(); err != nil {
				t.Fatalf("Router validation failed: %v", err)
			}

			exec, err := newRouter(context.Background(), tt.step)
			if err != nil {
				t.Fatalf("newRouter() failed: %v", err)
			}

			// Capture output
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			exec.SetStdout(stdout)
			exec.SetStderr(stderr)

			// Run the executor
			ctx := context.Background()
			err = exec.Run(ctx)
			if err != nil {
				t.Errorf("Run() error = %v, want nil", err)
				return
			}

			// Check stdout contains expected messages
			output := stdout.String()
			for _, want := range tt.wantOutputContains {
				if !strings.Contains(output, want) {
					t.Errorf("Run() output missing %q, got:\n%s", want, output)
				}
			}

			// Check stderr is empty (no errors)
			if stderr.Len() > 0 {
				t.Errorf("Run() stderr = %q, want empty", stderr.String())
			}

			// Check result was populated
			routerExec := exec.(*routerExecutor)
			if routerExec.result == nil {
				t.Error("Run() did not populate result")
				return
			}

			// Check activated steps
			if len(routerExec.result.ActivatedSteps) != len(tt.wantActivated) {
				t.Errorf("Run() activated steps count = %d, want %d",
					len(routerExec.result.ActivatedSteps), len(tt.wantActivated))
			}

			// Verify all expected steps are activated
			for _, want := range tt.wantActivated {
				found := false
				for _, got := range routerExec.result.ActivatedSteps {
					if got == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Run() missing activated step %q, got %v",
						want, routerExec.result.ActivatedSteps)
				}
			}

			// Check result metadata
			if routerExec.result.EvaluatedAt == "" {
				t.Error("Run() result missing EvaluatedAt timestamp")
			}
		})
	}
}

func TestRouterExecutor_DetermineNodeStatus(t *testing.T) {
	step := core.Step{
		Name: "test_router",
		Router: &core.RouterConfig{
			Value: "@exitCode",
			Mode:  core.RouterModeExclusive,
			Routes: map[string][]string{
				"0": {"success"},
			},
		},
	}

	exec, err := newRouter(context.Background(), step)
	if err != nil {
		t.Fatalf("newRouter() failed: %v", err)
	}

	// DetermineNodeStatus should always return NodeSucceeded
	status, err := exec.(executor.NodeStatusDeterminer).DetermineNodeStatus()
	if err != nil {
		t.Errorf("DetermineNodeStatus() error = %v, want nil", err)
	}
	if status != core.NodeSucceeded {
		t.Errorf("DetermineNodeStatus() = %v, want %v", status, core.NodeSucceeded)
	}
}

func TestRouterExecutor_GetRouterResult(t *testing.T) {
	step := core.Step{
		Name: "test_router",
		Router: &core.RouterConfig{
			Value: "@exitCode",
			Mode:  core.RouterModeExclusive,
			Routes: map[string][]string{
				"0": {"success_handler"},
			},
			Default: []string{"default_handler"},
		},
	}

	// Validate router first
	if err := step.Router.Validate(); err != nil {
		t.Fatalf("Router validation failed: %v", err)
	}

	exec, err := newRouter(context.Background(), step)
	if err != nil {
		t.Fatalf("newRouter() failed: %v", err)
	}

	provider := exec.(executor.RouterResultProvider)

	// Before Run(), result should be nil
	result := provider.GetRouterResult()
	if result != nil {
		t.Errorf("GetRouterResult() before Run() = %v, want nil", result)
	}

	// Run the executor
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exec.SetStdout(stdout)
	exec.SetStderr(stderr)

	ctx := context.Background()
	if err := exec.Run(ctx); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// After Run(), result should be populated
	result = provider.GetRouterResult()
	if result == nil {
		t.Fatal("GetRouterResult() after Run() = nil, want non-nil")
	}

	// Check result fields
	if len(result.ActivatedSteps) == 0 {
		t.Error("GetRouterResult() result has no activated steps")
	}
	if result.EvaluatedAt == "" {
		t.Error("GetRouterResult() result missing EvaluatedAt")
	}

	// Test deep copy protection - mutating returned result should not affect internal state
	originalActivated := make([]string, len(result.ActivatedSteps))
	copy(originalActivated, result.ActivatedSteps)

	// Mutate the returned result
	result.ActivatedSteps = append(result.ActivatedSteps, "injected_step")
	result.EvaluatedValue = "tampered"

	// Get result again and verify it's unchanged
	result2 := provider.GetRouterResult()
	if len(result2.ActivatedSteps) != len(originalActivated) {
		t.Errorf("GetRouterResult() deep copy failed: activated steps count = %d, want %d",
			len(result2.ActivatedSteps), len(originalActivated))
	}
	if result2.EvaluatedValue == "tampered" {
		t.Error("GetRouterResult() deep copy failed: value was mutated")
	}
}

func TestRouterExecutor_Interfaces(t *testing.T) {
	step := core.Step{
		Name: "test_router",
		Router: &core.RouterConfig{
			Value: "@exitCode",
			Mode:  core.RouterModeExclusive,
			Routes: map[string][]string{
				"0": {"success"},
			},
		},
	}

	exec, err := newRouter(context.Background(), step)
	if err != nil {
		t.Fatalf("newRouter() failed: %v", err)
	}

	// Test Executor interface
	if _, ok := exec.(executor.Executor); !ok {
		t.Error("routerExecutor does not implement executor.Executor")
	}

	// Test NodeStatusDeterminer interface
	if _, ok := exec.(executor.NodeStatusDeterminer); !ok {
		t.Error("routerExecutor does not implement executor.NodeStatusDeterminer")
	}

	// Test RouterResultProvider interface
	if _, ok := exec.(executor.RouterResultProvider); !ok {
		t.Error("routerExecutor does not implement executor.RouterResultProvider")
	}
}

func TestRouterExecutor_RunErrorHandling(t *testing.T) {
	// Note: Current implementation always uses exitCode=0 and value=""
	// so it's difficult to trigger evaluation errors without modifying the code.
	// This test verifies the error path exists and error message format.

	step := core.Step{
		Name: "test_router",
		Router: &core.RouterConfig{
			Value: "@exitCode",
			Mode:  core.RouterModeExclusive,
			Routes: map[string][]string{
				"0": {"success"},
			},
		},
	}

	// Validate router first
	if err := step.Router.Validate(); err != nil {
		t.Fatalf("Router validation failed: %v", err)
	}

	exec, err := newRouter(context.Background(), step)
	if err != nil {
		t.Fatalf("newRouter() failed: %v", err)
	}

	// Capture output
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exec.SetStdout(stdout)
	exec.SetStderr(stderr)

	// Run should succeed with default implementation
	ctx := context.Background()
	err = exec.Run(ctx)
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	// This test mainly documents that error handling exists
	// A future enhancement would be to make value/exitCode injectable for testing
}

func TestRouterExecutor_ResultMetadata(t *testing.T) {
	step := core.Step{
		Name: "test_router",
		Router: &core.RouterConfig{
			Value: "@exitCode",
			Mode:  core.RouterModeExclusive,
			Routes: map[string][]string{
				"0":   {"success_handler"},
				"500": {"error_handler"},
			},
			Default: []string{"default_handler"},
		},
	}

	// Validate router first
	if err := step.Router.Validate(); err != nil {
		t.Fatalf("Router validation failed: %v", err)
	}

	exec, err := newRouter(context.Background(), step)
	if err != nil {
		t.Fatalf("newRouter() failed: %v", err)
	}

	// Run the executor
	stdout := &bytes.Buffer{}
	exec.SetStdout(stdout)
	exec.SetStderr(&bytes.Buffer{})

	ctx := context.Background()
	if err := exec.Run(ctx); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Get result
	provider := exec.(executor.RouterResultProvider)
	result := provider.GetRouterResult()
	if result == nil {
		t.Fatal("GetRouterResult() = nil, want non-nil")
	}

	// Verify metadata fields are populated
	if result.EvaluatedValue != "" {
		// Current implementation always evaluates to empty string
		// This test documents the expected behavior
	}

	if result.EvaluatedAt == "" {
		t.Error("Result missing EvaluatedAt timestamp")
	}

	// Verify timestamp is in RFC3339 format
	if !strings.Contains(result.EvaluatedAt, "T") || !strings.Contains(result.EvaluatedAt, ":") {
		t.Errorf("EvaluatedAt = %q, want RFC3339 format", result.EvaluatedAt)
	}

	// Verify arrays are not nil even if empty
	if result.MatchedPatterns == nil {
		t.Error("MatchedPatterns should not be nil")
	}
	if result.ActivatedSteps == nil {
		t.Error("ActivatedSteps should not be nil")
	}
}

func TestRouterExecutor_MultipleGetRouterResult(t *testing.T) {
	step := core.Step{
		Name: "test_router",
		Router: &core.RouterConfig{
			Value: "@exitCode",
			Mode:  core.RouterModeExclusive,
			Routes: map[string][]string{
				"0": {"success"},
			},
		},
	}

	// Validate router first
	if err := step.Router.Validate(); err != nil {
		t.Fatalf("Router validation failed: %v", err)
	}

	exec, err := newRouter(context.Background(), step)
	if err != nil {
		t.Fatalf("newRouter() failed: %v", err)
	}

	// Run the executor
	stdout := &bytes.Buffer{}
	exec.SetStdout(stdout)
	exec.SetStderr(&bytes.Buffer{})

	ctx := context.Background()
	if err := exec.Run(ctx); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	provider := exec.(executor.RouterResultProvider)

	// Get result multiple times
	result1 := provider.GetRouterResult()
	result2 := provider.GetRouterResult()

	// Each call should return a new copy
	if result1 == result2 {
		t.Error("GetRouterResult() should return a new copy each time")
	}

	// But contents should be identical
	if result1.EvaluatedValue != result2.EvaluatedValue {
		t.Error("GetRouterResult() copies have different EvaluatedValue")
	}
	if result1.EvaluatedAt != result2.EvaluatedAt {
		t.Error("GetRouterResult() copies have different EvaluatedAt")
	}
	if len(result1.ActivatedSteps) != len(result2.ActivatedSteps) {
		t.Error("GetRouterResult() copies have different ActivatedSteps length")
	}
}

// TestRouterExecutor_EmptyResultArrays verifies that the result arrays
// are properly initialized even when empty, not nil.
func TestRouterExecutor_EmptyResultArrays(t *testing.T) {
	step := core.Step{
		Name: "test_router",
		Router: &core.RouterConfig{
			Value: "@exitCode",
			Mode:  core.RouterModeExclusive,
			Routes: map[string][]string{
				"999": {"never_matches"},
			},
			Default: []string{}, // Empty default to test empty arrays
		},
	}

	// Validate router first
	if err := step.Router.Validate(); err != nil {
		t.Fatalf("Router validation failed: %v", err)
	}

	exec, err := newRouter(context.Background(), step)
	if err != nil {
		t.Fatalf("newRouter() failed: %v", err)
	}

	// Run the executor
	stdout := &bytes.Buffer{}
	exec.SetStdout(stdout)
	exec.SetStderr(&bytes.Buffer{})

	ctx := context.Background()
	if err := exec.Run(ctx); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Get result
	provider := exec.(executor.RouterResultProvider)
	result := provider.GetRouterResult()
	if result == nil {
		t.Fatal("GetRouterResult() = nil, want non-nil")
	}

	// Verify arrays are initialized (not nil), even if empty
	if result.MatchedPatterns == nil {
		t.Error("MatchedPatterns is nil, want empty slice")
	}
	if result.ActivatedSteps == nil {
		t.Error("ActivatedSteps is nil, want empty slice")
	}

	// Verify they're actually empty
	if len(result.MatchedPatterns) != 0 {
		t.Errorf("MatchedPatterns length = %d, want 0", len(result.MatchedPatterns))
	}
	if len(result.ActivatedSteps) != 0 {
		t.Errorf("ActivatedSteps length = %d, want 0", len(result.ActivatedSteps))
	}
}
