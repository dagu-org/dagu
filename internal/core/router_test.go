package core

import (
	"testing"
)

func TestRouterConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		router  *RouterConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid exclusive router with regex patterns",
			router: &RouterConfig{
				Value: "@exitCode",
				Mode:  RouterModeExclusive,
				Routes: map[string][]string{
					"/^5[0-9]{2}$/": {"handle_server_error"},
					"/^4[0-9]{2}$/": {"handle_client_error"},
					"0":             {"success"},
				},
				Default: []string{"handle_unknown"},
			},
			wantErr: false,
		},
		{
			name: "valid multi-select router",
			router: &RouterConfig{
				Value: "@value",
				Mode:  RouterModeMultiSelect,
				Routes: map[string][]string{
					"error":   {"log_error", "notify_team"},
					"warning": {"log_warning"},
				},
				Default: []string{"default_handler"},
			},
			wantErr: false,
		},
		{
			name: "valid router with exit code array",
			router: &RouterConfig{
				Value: "@exitCode",
				Mode:  RouterModeExclusive,
				Routes: map[string][]string{
					"[500, 502, 503]": {"retry_step"},
					"[400, 401, 403]": {"auth_error"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid router with expression",
			router: &RouterConfig{
				Value: "@value",
				Mode:  RouterModeExclusive,
				Routes: map[string][]string{
					"@exitCode > 0": {"handle_error"},
					"@exitCode == 0": {"success"},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid regex pattern - unclosed bracket",
			router: &RouterConfig{
				Value: "@exitCode",
				Mode:  RouterModeExclusive,
				Routes: map[string][]string{
					"/^5[0-9{2}$/": {"handle_error"},
				},
			},
			wantErr: true,
			errMsg:  "invalid regex pattern",
		},
		{
			name: "invalid exit code array - not numeric",
			router: &RouterConfig{
				Value: "@exitCode",
				Mode:  RouterModeExclusive,
				Routes: map[string][]string{
					"[500, abc, 503]": {"retry_step"},
				},
			},
			wantErr: true,
			errMsg:  "invalid exit code array",
		},
		{
			name: "invalid mode",
			router: &RouterConfig{
				Value: "@exitCode",
				Mode:  "invalid_mode",
				Routes: map[string][]string{
					"0": {"success"},
				},
			},
			wantErr: true,
			errMsg:  "invalid router mode",
		},
		{
			name: "empty value field",
			router: &RouterConfig{
				Value: "",
				Mode:  RouterModeExclusive,
				Routes: map[string][]string{
					"0": {"success"},
				},
			},
			wantErr: true,
			errMsg:  "router value cannot be empty",
		},
		{
			name: "no routes and no default",
			router: &RouterConfig{
				Value:  "@exitCode",
				Mode:   RouterModeExclusive,
				Routes: map[string][]string{},
			},
			wantErr: true,
			errMsg:  "router must have at least one route or default",
		},
		{
			name: "ReDoS protection - catastrophic backtracking pattern",
			router: &RouterConfig{
				Value: "@value",
				Mode:  RouterModeExclusive,
				Routes: map[string][]string{
					"/^(a+)+$/": {"match"},
				},
			},
			wantErr: true,
			errMsg:  "potentially unsafe regex pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.router.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errMsg != "" && err.Error() == "" {
					t.Errorf("Validate() error message is empty, want %q", tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				}
			}
		})
	}
}

func TestRouterConfig_EvaluateRoutes(t *testing.T) {
	tests := []struct {
		name              string
		router            *RouterConfig
		value             string
		exitCode          int
		wantPatterns      []string
		wantActivated     []string
		wantErr           bool
	}{
		{
			name: "exclusive mode - exit code match",
			router: &RouterConfig{
				Value: "@exitCode",
				Mode:  RouterModeExclusive,
				Routes: map[string][]string{
					"0":   {"success_handler"},
					"1":   {"error_handler"},
					"500": {"server_error_handler"},
				},
				Default: []string{"default_handler"},
			},
			value:         "",
			exitCode:      500,
			wantPatterns:  []string{"500"},
			wantActivated: []string{"server_error_handler"},
			wantErr:       false,
		},
		{
			name: "exclusive mode - regex match",
			router: &RouterConfig{
				Value: "@value",
				Mode:  RouterModeExclusive,
				Routes: map[string][]string{
					"/^error:.*/":   {"error_handler"},
					"/^warning:.*/": {"warning_handler"},
					"success":       {"success_handler"},
				},
				Default: []string{"default_handler"},
			},
			value:         "error:database_connection_failed",
			exitCode:      1,
			wantPatterns:  []string{"/^error:.*/"},
			wantActivated: []string{"error_handler"},
			wantErr:       false,
		},
		{
			name: "exclusive mode - exit code array match",
			router: &RouterConfig{
				Value: "@exitCode",
				Mode:  RouterModeExclusive,
				Routes: map[string][]string{
					"[500, 502, 503]": {"retry_handler"},
					"[400, 401]":      {"auth_handler"},
					"0":               {"success_handler"},
				},
			},
			value:         "",
			exitCode:      502,
			wantPatterns:  []string{"[500, 502, 503]"},
			wantActivated: []string{"retry_handler"},
			wantErr:       false,
		},
		{
			name: "exclusive mode - expression match",
			router: &RouterConfig{
				Value: "@exitCode",
				Mode:  RouterModeExclusive,
				Routes: map[string][]string{
					"@exitCode > 0":  {"error_handler"},
					"@exitCode == 0": {"success_handler"},
				},
			},
			value:         "",
			exitCode:      1,
			wantPatterns:  []string{"@exitCode > 0"},
			wantActivated: []string{"error_handler"},
			wantErr:       false,
		},
		{
			name: "multi-select mode - multiple matches",
			router: &RouterConfig{
				Value: "@value",
				Mode:  RouterModeMultiSelect,
				Routes: map[string][]string{
					"/error/":   {"log_error"},
					"/.*fail.*/": {"notify_team"},
					"/database/": {"check_db"},
				},
			},
			value:         "database_error_connection_failed",
			exitCode:      1,
			wantPatterns:  []string{"/error/", "/.*fail.*/", "/database/"},
			wantActivated: []string{"log_error", "notify_team", "check_db"},
			wantErr:       false,
		},
		{
			name: "exclusive mode - no match uses default",
			router: &RouterConfig{
				Value: "@exitCode",
				Mode:  RouterModeExclusive,
				Routes: map[string][]string{
					"1": {"error_handler"},
					"2": {"warning_handler"},
				},
				Default: []string{"default_handler", "log_unknown"},
			},
			value:         "",
			exitCode:      99,
			wantPatterns:  nil,
			wantActivated: []string{"default_handler", "log_unknown"},
			wantErr:       false,
		},
		{
			name: "multi-select mode - no match uses default",
			router: &RouterConfig{
				Value: "@value",
				Mode:  RouterModeMultiSelect,
				Routes: map[string][]string{
					"error": {"error_handler"},
				},
				Default: []string{"default_handler"},
			},
			value:         "info message",
			exitCode:      0,
			wantPatterns:  nil,
			wantActivated: []string{"default_handler"},
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate first (required before evaluation)
			if err := tt.router.Validate(); err != nil {
				t.Fatalf("Validate() failed: %v", err)
			}

			gotPatterns, gotActivated, err := tt.router.EvaluateRoutes(tt.value, tt.exitCode)

			if tt.wantErr {
				if err == nil {
					t.Errorf("EvaluateRoutes() error = nil, wantErr %v", tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("EvaluateRoutes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Check matched patterns
			if len(gotPatterns) != len(tt.wantPatterns) {
				t.Errorf("EvaluateRoutes() patterns count = %d, want %d", len(gotPatterns), len(tt.wantPatterns))
			}

			// Check activated steps
			if len(gotActivated) != len(tt.wantActivated) {
				t.Errorf("EvaluateRoutes() activated count = %d, want %d", len(gotActivated), len(tt.wantActivated))
			}

			// Verify all expected steps are activated
			for _, want := range tt.wantActivated {
				found := false
				for _, got := range gotActivated {
					if got == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("EvaluateRoutes() missing activated step %q, got %v", want, gotActivated)
				}
			}
		})
	}
}

func TestStep_IsRouter(t *testing.T) {
	tests := []struct {
		name string
		step *Step
		want bool
	}{
		{
			name: "step with router config",
			step: &Step{
				Name: "router_step",
				Router: &RouterConfig{
					Value: "@exitCode",
					Mode:  RouterModeExclusive,
					Routes: map[string][]string{
						"0": {"success"},
					},
				},
			},
			want: true,
		},
		{
			name: "step without router config",
			step: &Step{
				Name:    "normal_step",
				Command: "echo hello",
			},
			want: false,
		},
		{
			name: "step with nil router",
			step: &Step{
				Name:   "nil_router",
				Router: nil,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.step.IsRouter(); got != tt.want {
				t.Errorf("Step.IsRouter() = %v, want %v", got, tt.want)
			}
		})
	}
}
