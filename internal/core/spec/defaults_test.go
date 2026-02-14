package spec

import (
	"testing"

	"github.com/dagu-org/dagu/internal/core/spec/types"
	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyDefaults(t *testing.T) {
	t.Parallel()

	boolPtr := func(b bool) *bool { return &b }
	strPtr := func(s string) *string { return &s }

	t.Run("NilDefaults", func(t *testing.T) {
		t.Parallel()
		s := &step{TimeoutSec: 10}
		applyDefaults(s, nil)
		assert.Equal(t, 10, s.TimeoutSec)
	})

	t.Run("EmptyDefaults", func(t *testing.T) {
		t.Parallel()
		s := &step{TimeoutSec: 10}
		applyDefaults(s, &defaults{})
		assert.Equal(t, 10, s.TimeoutSec)
	})

	t.Run("RetryPolicy_Inherits", func(t *testing.T) {
		t.Parallel()
		rp := &retryPolicy{Limit: 3, IntervalSec: 5}
		s := &step{}
		applyDefaults(s, &defaults{RetryPolicy: rp})
		assert.Equal(t, rp, s.RetryPolicy)
	})

	t.Run("RetryPolicy_StepOverrides", func(t *testing.T) {
		t.Parallel()
		stepRP := &retryPolicy{Limit: 10}
		defRP := &retryPolicy{Limit: 3, IntervalSec: 5}
		s := &step{RetryPolicy: stepRP}
		applyDefaults(s, &defaults{RetryPolicy: defRP})
		assert.Equal(t, stepRP, s.RetryPolicy)
		assert.Equal(t, 10, s.RetryPolicy.Limit.(int))
	})

	t.Run("ContinueOn_Inherits", func(t *testing.T) {
		t.Parallel()
		co := continueOnValue("failed")
		s := &step{}
		applyDefaults(s, &defaults{ContinueOn: co})
		assert.False(t, s.ContinueOn.IsZero())
		assert.True(t, s.ContinueOn.Failed())
	})

	t.Run("ContinueOn_StepOverrides", func(t *testing.T) {
		t.Parallel()
		stepCO := continueOnValue("skipped")
		defCO := continueOnValue("failed")
		s := &step{ContinueOn: stepCO}
		applyDefaults(s, &defaults{ContinueOn: defCO})
		assert.True(t, s.ContinueOn.Skipped())
		assert.False(t, s.ContinueOn.Failed())
	})

	t.Run("RepeatPolicy_Inherits", func(t *testing.T) {
		t.Parallel()
		rp := &repeatPolicy{Repeat: "while", IntervalSec: 30}
		s := &step{}
		applyDefaults(s, &defaults{RepeatPolicy: rp})
		assert.Equal(t, rp, s.RepeatPolicy)
	})

	t.Run("RepeatPolicy_StepOverrides", func(t *testing.T) {
		t.Parallel()
		stepRP := &repeatPolicy{Repeat: "until", IntervalSec: 10}
		defRP := &repeatPolicy{Repeat: "while", IntervalSec: 30}
		s := &step{RepeatPolicy: stepRP}
		applyDefaults(s, &defaults{RepeatPolicy: defRP})
		assert.Equal(t, stepRP, s.RepeatPolicy)
	})

	t.Run("TimeoutSec_Inherits", func(t *testing.T) {
		t.Parallel()
		s := &step{}
		applyDefaults(s, &defaults{TimeoutSec: 600})
		assert.Equal(t, 600, s.TimeoutSec)
	})

	t.Run("TimeoutSec_StepOverrides", func(t *testing.T) {
		t.Parallel()
		s := &step{TimeoutSec: 300}
		applyDefaults(s, &defaults{TimeoutSec: 600})
		assert.Equal(t, 300, s.TimeoutSec)
	})

	t.Run("MailOnError_Inherits_True", func(t *testing.T) {
		t.Parallel()
		s := &step{}
		applyDefaults(s, &defaults{MailOnError: boolPtr(true)})
		assert.True(t, s.MailOnError)
	})

	t.Run("MailOnError_NilDefault_NoChange", func(t *testing.T) {
		t.Parallel()
		s := &step{}
		applyDefaults(s, &defaults{MailOnError: nil})
		assert.False(t, s.MailOnError)
	})

	t.Run("MailOnError_StepTrue_DefaultFalse", func(t *testing.T) {
		t.Parallel()
		s := &step{MailOnError: true}
		applyDefaults(s, &defaults{MailOnError: boolPtr(false)})
		// Step already has true, default false doesn't override
		assert.True(t, s.MailOnError)
	})

	t.Run("SignalOnStop_Inherits", func(t *testing.T) {
		t.Parallel()
		s := &step{}
		applyDefaults(s, &defaults{SignalOnStop: strPtr("SIGTERM")})
		require.NotNil(t, s.SignalOnStop)
		assert.Equal(t, "SIGTERM", *s.SignalOnStop)
	})

	t.Run("SignalOnStop_StepOverrides", func(t *testing.T) {
		t.Parallel()
		s := &step{SignalOnStop: strPtr("SIGINT")}
		applyDefaults(s, &defaults{SignalOnStop: strPtr("SIGTERM")})
		assert.Equal(t, "SIGINT", *s.SignalOnStop)
	})

	t.Run("Env_Inherits_WhenStepEmpty", func(t *testing.T) {
		t.Parallel()
		defEnv := envValueMap(map[string]string{"LOG_LEVEL": "info"})
		s := &step{}
		applyDefaults(s, &defaults{Env: defEnv})
		require.False(t, s.Env.IsZero())
		entries := s.Env.Entries()
		require.Len(t, entries, 1)
		assert.Equal(t, "LOG_LEVEL", entries[0].Key)
		assert.Equal(t, "info", entries[0].Value)
	})

	t.Run("Env_Additive", func(t *testing.T) {
		t.Parallel()
		defEnv := envValueMap(map[string]string{"LOG_LEVEL": "info"})
		stepEnv := envValueMap(map[string]string{"EXTRA": "true"})
		s := &step{Env: stepEnv}
		applyDefaults(s, &defaults{Env: defEnv})

		entries := s.Env.Entries()
		require.Len(t, entries, 2)
		// Default entry comes first
		assert.Equal(t, "LOG_LEVEL", entries[0].Key)
		assert.Equal(t, "EXTRA", entries[1].Key)
	})

	t.Run("Preconditions_Inherits_WhenStepNil", func(t *testing.T) {
		t.Parallel()
		defPrecond := "test -f /tmp/ready"
		s := &step{}
		applyDefaults(s, &defaults{Preconditions: defPrecond})
		assert.Equal(t, defPrecond, s.Preconditions)
	})

	t.Run("Preconditions_Additive_StringAndString", func(t *testing.T) {
		t.Parallel()
		s := &step{Preconditions: "test -d /data"}
		applyDefaults(s, &defaults{Preconditions: "test -f /tmp/ready"})
		combined, ok := s.Preconditions.([]any)
		require.True(t, ok)
		assert.Len(t, combined, 2)
		assert.Equal(t, "test -f /tmp/ready", combined[0])
		assert.Equal(t, "test -d /data", combined[1])
	})

	t.Run("Preconditions_Additive_ArrayAndArray", func(t *testing.T) {
		t.Parallel()
		defPrecond := []any{"check1", "check2"}
		stepPrecond := []any{"check3"}
		s := &step{Preconditions: stepPrecond}
		applyDefaults(s, &defaults{Preconditions: defPrecond})
		combined, ok := s.Preconditions.([]any)
		require.True(t, ok)
		assert.Len(t, combined, 3)
		assert.Equal(t, "check1", combined[0])
		assert.Equal(t, "check2", combined[1])
		assert.Equal(t, "check3", combined[2])
	})

	t.Run("AllFieldsSimultaneously", func(t *testing.T) {
		t.Parallel()
		d := &defaults{
			RetryPolicy:   &retryPolicy{Limit: 3, IntervalSec: 5},
			ContinueOn:    continueOnValue("failed"),
			RepeatPolicy:  &repeatPolicy{Repeat: "while", IntervalSec: 30},
			TimeoutSec:    600,
			MailOnError:   boolPtr(true),
			SignalOnStop:  strPtr("SIGTERM"),
			Env:           envValueMap(map[string]string{"DEFAULT_VAR": "value"}),
			Preconditions: "test -f /ready",
		}

		s := &step{}
		applyDefaults(s, d)

		assert.NotNil(t, s.RetryPolicy)
		assert.False(t, s.ContinueOn.IsZero())
		assert.NotNil(t, s.RepeatPolicy)
		assert.Equal(t, 600, s.TimeoutSec)
		assert.True(t, s.MailOnError)
		require.NotNil(t, s.SignalOnStop)
		assert.Equal(t, "SIGTERM", *s.SignalOnStop)
		assert.False(t, s.Env.IsZero())
		assert.Equal(t, "test -f /ready", s.Preconditions)
	})
}

func TestDecodeDefaults(t *testing.T) {
	t.Parallel()

	t.Run("NilInput", func(t *testing.T) {
		t.Parallel()
		d, err := decodeDefaults(nil)
		require.NoError(t, err)
		assert.Nil(t, d)
	})

	t.Run("ValidAllFields", func(t *testing.T) {
		t.Parallel()
		raw := map[string]any{
			"retry_policy": map[string]any{
				"limit":        3,
				"interval_sec": 5,
			},
			"continue_on":   "failed",
			"repeat_policy": map[string]any{"repeat": "while", "interval_sec": 30},
			"timeout_sec":   600,
			"mail_on_error": true,
			"signal_on_stop": "SIGTERM",
			"env": []any{"LOG_LEVEL=info"},
			"preconditions": "test -f /ready",
		}

		d, err := decodeDefaults(raw)
		require.NoError(t, err)
		require.NotNil(t, d)

		assert.NotNil(t, d.RetryPolicy)
		assert.Equal(t, 3, d.RetryPolicy.Limit)
		assert.False(t, d.ContinueOn.IsZero())
		assert.NotNil(t, d.RepeatPolicy)
		assert.Equal(t, 600, d.TimeoutSec)
		require.NotNil(t, d.MailOnError)
		assert.True(t, *d.MailOnError)
		require.NotNil(t, d.SignalOnStop)
		assert.Equal(t, "SIGTERM", *d.SignalOnStop)
		assert.False(t, d.Env.IsZero())
		assert.Equal(t, "test -f /ready", d.Preconditions)
	})

	t.Run("UnknownKey_Error", func(t *testing.T) {
		t.Parallel()
		raw := map[string]any{
			"timeout_sec":   600,
			"unknown_field": "value",
		}

		d, err := decodeDefaults(raw)
		assert.Error(t, err)
		assert.Nil(t, d)
		assert.Contains(t, err.Error(), "defaults")
	})

	t.Run("EmptyMap", func(t *testing.T) {
		t.Parallel()
		raw := map[string]any{}

		d, err := decodeDefaults(raw)
		require.NoError(t, err)
		require.NotNil(t, d)
		// All fields at zero values
		assert.Nil(t, d.RetryPolicy)
		assert.True(t, d.ContinueOn.IsZero())
		assert.Equal(t, 0, d.TimeoutSec)
	})
}

func TestCombinePreconditions(t *testing.T) {
	t.Parallel()

	t.Run("StringAndString", func(t *testing.T) {
		t.Parallel()
		result := combinePreconditions("a", "b")
		assert.Equal(t, []any{"a", "b"}, result)
	})

	t.Run("ArrayAndArray", func(t *testing.T) {
		t.Parallel()
		result := combinePreconditions([]any{"a", "b"}, []any{"c"})
		assert.Equal(t, []any{"a", "b", "c"}, result)
	})

	t.Run("StringAndArray", func(t *testing.T) {
		t.Parallel()
		result := combinePreconditions("a", []any{"b", "c"})
		assert.Equal(t, []any{"a", "b", "c"}, result)
	})

	t.Run("ArrayAndString", func(t *testing.T) {
		t.Parallel()
		result := combinePreconditions([]any{"a", "b"}, "c")
		assert.Equal(t, []any{"a", "b", "c"}, result)
	})

	t.Run("MapAndString", func(t *testing.T) {
		t.Parallel()
		m := map[string]any{"condition": "test", "expected": "ok"}
		result := combinePreconditions(m, "check")
		assert.Len(t, result, 2)
		assert.Equal(t, m, result[0])
		assert.Equal(t, "check", result[1])
	})
}

// Helper to create EnvValue from YAML
func testEnvValue(yamlStr string) types.EnvValue {
	var v types.EnvValue
	_ = yaml.Unmarshal([]byte(yamlStr), &v)
	return v
}
