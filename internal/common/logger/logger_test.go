package logger

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestLogger_SourceLocation(t *testing.T) {
	tests := []struct {
		name          string
		logFunc       func(Logger)
		expectedInLog string
		shouldNotHave []string
	}{
		{
			name: "InfoMethodShowsCorrectSource",
			logFunc: func(l Logger) {
				l.Info("test message")
			},
			expectedInLog: "logger_test.go:",
			shouldNotHave: []string{"internal/logger/logger.go", "slog-multi"},
		},
		{
			name: "DebugMethodShowsCorrectSource",
			logFunc: func(l Logger) {
				l.Debug("debug message")
			},
			expectedInLog: "logger_test.go:",
			shouldNotHave: []string{"internal/logger/logger.go", "slog-multi"},
		},
		{
			name: "ErrorMethodShowsCorrectSource",
			logFunc: func(l Logger) {
				l.Error("error message")
			},
			expectedInLog: "logger_test.go:",
			shouldNotHave: []string{"internal/logger/logger.go", "slog-multi"},
		},
		{
			name: "WarnMethodShowsCorrectSource",
			logFunc: func(l Logger) {
				l.Warn("warn message")
			},
			expectedInLog: "logger_test.go:",
			shouldNotHave: []string{"internal/logger/logger.go", "slog-multi"},
		},
		{
			name: "InfofMethodShowsCorrectSource",
			logFunc: func(l Logger) {
				l.Infof("formatted %s", "message")
			},
			expectedInLog: "logger_test.go:",
			shouldNotHave: []string{"internal/logger/logger.go", "slog-multi"},
		},
		{
			name: "DebugfMethodShowsCorrectSource",
			logFunc: func(l Logger) {
				l.Debugf("debug %d", 42)
			},
			expectedInLog: "logger_test.go:",
			shouldNotHave: []string{"internal/logger/logger.go", "slog-multi"},
		},
		{
			name: "ErrorfMethodShowsCorrectSource",
			logFunc: func(l Logger) {
				l.Errorf("error %v", "test")
			},
			expectedInLog: "logger_test.go:",
			shouldNotHave: []string{"internal/logger/logger.go", "slog-multi"},
		},
		{
			name: "WarnfMethodShowsCorrectSource",
			logFunc: func(l Logger) {
				l.Warnf("warning %s", "test")
			},
			expectedInLog: "logger_test.go:",
			shouldNotHave: []string{"internal/logger/logger.go", "slog-multi"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewLogger(
				WithDebug(),
				WithFormat("text"),
				WithWriter(&buf),
				WithQuiet(),
			)

			tt.logFunc(logger)

			output := buf.String()

			// Check that the expected source location is present
			if !strings.Contains(output, tt.expectedInLog) {
				t.Errorf("Expected log to contain %q, but got: %s", tt.expectedInLog, output)
			}

			// Check that unwanted locations are not present
			for _, shouldNotHave := range tt.shouldNotHave {
				if strings.Contains(output, shouldNotHave) {
					t.Errorf("Log should not contain %q, but got: %s", shouldNotHave, output)
				}
			}
		})
	}
}

func TestLogger_SourceLocationWithContext(t *testing.T) {
	tests := []struct {
		name          string
		logFunc       func(context.Context)
		expectedInLog string
		shouldNotHave []string
	}{
		{
			name: "ContextInfoShowsCorrectSource",
			logFunc: func(ctx context.Context) {
				Info(ctx, "context info message")
			},
			expectedInLog: "logger_test.go:",
			shouldNotHave: []string{"internal/logger/logger.go", "internal/logger/context.go", "slog-multi"},
		},
		{
			name: "ContextDebugShowsCorrectSource",
			logFunc: func(ctx context.Context) {
				Debug(ctx, "context debug message")
			},
			expectedInLog: "logger_test.go:",
			shouldNotHave: []string{"internal/logger/logger.go", "internal/logger/context.go", "slog-multi"},
		},
		{
			name: "ContextErrorShowsCorrectSource",
			logFunc: func(ctx context.Context) {
				Error(ctx, "context error message")
			},
			expectedInLog: "logger_test.go:",
			shouldNotHave: []string{"internal/logger/logger.go", "internal/logger/context.go", "slog-multi"},
		},
		{
			name: "ContextWarnShowsCorrectSource",
			logFunc: func(ctx context.Context) {
				Warn(ctx, "context warn message")
			},
			expectedInLog: "logger_test.go:",
			shouldNotHave: []string{"internal/logger/logger.go", "internal/logger/context.go", "slog-multi"},
		},
		{
			name: "ContextInfofShowsCorrectSource",
			logFunc: func(ctx context.Context) {
				Infof(ctx, "formatted %s", "context")
			},
			expectedInLog: "logger_test.go:",
			shouldNotHave: []string{"internal/logger/logger.go", "internal/logger/context.go", "slog-multi"},
		},
		{
			name: "ContextDebugfShowsCorrectSource",
			logFunc: func(ctx context.Context) {
				Debugf(ctx, "debug %d", 123)
			},
			expectedInLog: "logger_test.go:",
			shouldNotHave: []string{"internal/logger/logger.go", "internal/logger/context.go", "slog-multi"},
		},
		{
			name: "ContextErrorfShowsCorrectSource",
			logFunc: func(ctx context.Context) {
				Errorf(ctx, "error %v", "context")
			},
			expectedInLog: "logger_test.go:",
			shouldNotHave: []string{"internal/logger/logger.go", "internal/logger/context.go", "slog-multi"},
		},
		{
			name: "ContextWarnfShowsCorrectSource",
			logFunc: func(ctx context.Context) {
				Warnf(ctx, "warning %s", "context")
			},
			expectedInLog: "logger_test.go:",
			shouldNotHave: []string{"internal/logger/logger.go", "internal/logger/context.go", "slog-multi"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewLogger(
				WithDebug(),
				WithFormat("text"),
				WithWriter(&buf),
				WithQuiet(),
			)

			ctx := WithLogger(context.Background(), logger)
			tt.logFunc(ctx)

			output := buf.String()

			// Check that the expected source location is present
			if !strings.Contains(output, tt.expectedInLog) {
				t.Errorf("Expected log to contain %q, but got: %s", tt.expectedInLog, output)
			}

			// Check that unwanted locations are not present
			for _, shouldNotHave := range tt.shouldNotHave {
				if strings.Contains(output, shouldNotHave) {
					t.Errorf("Log should not contain %q, but got: %s", shouldNotHave, output)
				}
			}
		})
	}
}

func TestLogger_SourceLocationWithNestedCalls(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(
		WithDebug(),
		WithFormat("text"),
		WithWriter(&buf),
		WithQuiet(),
	)

	// Helper function that logs
	logHelper := func(l Logger) {
		l.Info("from helper")
	}

	// Another level of nesting
	outerHelper := func(l Logger) {
		logHelper(l) // This calls the helper
	}

	// Call through nested functions
	outerHelper(logger)
	output := buf.String()

	// Should show the actual logging location in logHelper, not internal/logger/logger.go
	if strings.Contains(output, "internal/logger/logger.go") {
		t.Errorf("Log should not contain internal/logger/logger.go, but got: %s", output)
	}

	// Should contain logger_test.go (from the helper function)
	if !strings.Contains(output, "logger_test.go") {
		t.Errorf("Expected log to contain logger_test.go, but got: %s", output)
	}
}

func TestLogger_SourceLocationWithAttributes(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(
		WithDebug(),
		WithFormat("text"),
		WithWriter(&buf),
		WithQuiet(),
	)

	// Test with attributes
	logger.With("key", "value").Info("with attributes")

	output := buf.String()

	// Should not show internal logger location even with attributes
	if strings.Contains(output, "internal/logger/logger.go") {
		t.Errorf("Log should not contain internal/logger/logger.go, but got: %s", output)
	}

	// Should show this test file
	if !strings.Contains(output, "logger_test.go") {
		t.Errorf("Expected log to contain logger_test.go, but got: %s", output)
	}
}

func TestLogger_SourceLocationWithGroup(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(
		WithDebug(),
		WithFormat("text"),
		WithWriter(&buf),
		WithQuiet(),
	)

	// Test with group
	logger.WithGroup("test-group").Info("with group")

	output := buf.String()

	// Should not show internal logger location even with groups
	if strings.Contains(output, "internal/logger/logger.go") {
		t.Errorf("Log should not contain internal/logger/logger.go, but got: %s", output)
	}

	// Should show this test file
	if !strings.Contains(output, "logger_test.go") {
		t.Errorf("Expected log to contain logger_test.go, but got: %s", output)
	}
}

func TestLogger_SourceLocationDisabledInProduction(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(
		// No WithDebug() - production mode
		WithFormat("text"),
		WithWriter(&buf),
		WithQuiet(),
	)

	logger.Info("production mode")

	output := buf.String()

	// Should not contain source information in production mode
	if strings.Contains(output, "source=") {
		t.Errorf("Log should not contain source information in production mode, but got: %s", output)
	}
}

func TestLogger_JSONFormatSourceLocation(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(
		WithDebug(),
		WithFormat("json"),
		WithWriter(&buf),
		WithQuiet(),
	)

	logger.Info("json format test")

	output := buf.String()

	// JSON format should also not show internal logger location
	if strings.Contains(output, "internal/logger/logger.go") ||
		strings.Contains(output, "internal\\/logger\\/logger.go") {
		t.Errorf("JSON log should not contain internal/logger/logger.go, but got: %s", output)
	}

	// Should contain this test file
	if !strings.Contains(output, "logger_test.go") {
		t.Errorf("Expected JSON log to contain logger_test.go, but got: %s", output)
	}
}
