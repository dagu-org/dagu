package spec

import (
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/spec/types"
	"github.com/go-viper/mapstructure/v2"
)

// defaults defines the default values for step configuration fields.
// These are applied to every step that does not explicitly set its own value.
type defaults struct {
	ContinueOn    types.ContinueOnValue `yaml:"continue_on,omitempty"`
	RetryPolicy   *retryPolicy          `yaml:"retry_policy,omitempty"`
	RepeatPolicy  *repeatPolicy         `yaml:"repeat_policy,omitempty"`
	TimeoutSec    int                   `yaml:"timeout_sec,omitempty"`
	MailOnError   *bool                 `yaml:"mail_on_error,omitempty"`
	SignalOnStop  *string               `yaml:"signal_on_stop,omitempty"`
	Env           types.EnvValue        `yaml:"env,omitempty"`
	Preconditions any                   `yaml:"preconditions,omitempty"`
	Agent         *agentDefaults        `yaml:"agent,omitempty"`
}

// agentDefaults defines default values for agent step configuration.
// Fields mirror agentConfig; each is applied only when the step does not
// explicitly set its own value.
type agentDefaults struct {
	Model         string             `yaml:"model,omitempty"`
	Tools         *agentToolsConfig  `yaml:"tools,omitempty"`
	Skills        []string           `yaml:"skills,omitempty"`
	Soul          string             `yaml:"soul,omitempty"`
	Memory        *agentMemoryConfig `yaml:"memory,omitempty"`
	Prompt        string             `yaml:"prompt,omitempty"`
	MaxIterations *int               `yaml:"max_iterations,omitempty"`
	SafeMode      *bool              `yaml:"safe_mode,omitempty"`
}

// decodeDefaults decodes a raw value (from YAML) into a typed *defaults struct.
// Returns nil if raw is nil. Unknown keys cause a validation error.
func decodeDefaults(raw any) (*defaults, error) {
	if raw == nil {
		return nil, nil
	}
	d := new(defaults)
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		ErrorUnused: true,
		Result:      d,
		TagName:     "yaml",
		DecodeHook:  TypedUnionDecodeHook(),
	})
	if err != nil {
		return nil, core.NewValidationError("defaults", raw, err)
	}
	if err := decoder.Decode(raw); err != nil {
		return nil, core.NewValidationError("defaults", raw, withSnakeCaseKeyHint(err))
	}
	return d, nil
}

// applyDefaults merges default values into a step. Override fields are set only
// when the step did not explicitly set its own value. Additive fields (env,
// preconditions) prepend default entries before the step's own entries.
//
// The raw parameter is the original map[string]any from YAML decoding. When
// available, key presence in raw is used to detect explicit values (including
// zero values like timeout_sec: 0 or mail_on_error: false). When raw is nil
// (e.g. handler steps), the function falls back to checking the step field's
// Go zero value.
func applyDefaults(s *step, d *defaults, raw map[string]any) {
	if d == nil {
		return
	}

	// shouldApply reports whether the default for key should be applied.
	// When raw is available, checks that the key was NOT explicitly set in YAML.
	// When raw is nil, falls back to checking the step field's zero value.
	shouldApply := func(key string, isZero bool) bool {
		if raw != nil {
			_, ok := raw[key]
			return !ok
		}
		return isZero
	}

	// Override fields: apply only if step did not explicitly set the field
	if shouldApply("retry_policy", s.RetryPolicy == nil) && d.RetryPolicy != nil {
		s.RetryPolicy = d.RetryPolicy
	}
	if shouldApply("continue_on", s.ContinueOn.IsZero()) && !d.ContinueOn.IsZero() {
		s.ContinueOn = d.ContinueOn
	}
	if shouldApply("repeat_policy", s.RepeatPolicy == nil) && d.RepeatPolicy != nil {
		s.RepeatPolicy = d.RepeatPolicy
	}
	if shouldApply("timeout_sec", s.TimeoutSec == 0) && d.TimeoutSec != 0 {
		s.TimeoutSec = d.TimeoutSec
	}
	if shouldApply("mail_on_error", !s.MailOnError) && d.MailOnError != nil {
		s.MailOnError = *d.MailOnError
	}
	if shouldApply("signal_on_stop", s.SignalOnStop == nil) && d.SignalOnStop != nil {
		s.SignalOnStop = d.SignalOnStop
	}

	// Agent defaults: apply each field only if the step doesn't set it.
	// Like the top-level shouldApply, we consult the raw YAML map so that
	// explicit zero values (e.g. soul: "") are honoured and not overridden.
	if d.Agent != nil {
		if s.Agent == nil {
			s.Agent = &agentConfig{}
		}
		a, da := s.Agent, d.Agent

		var agentRaw map[string]any
		if raw != nil {
			if v, ok := raw["agent"].(map[string]any); ok {
				agentRaw = v
			}
		}

		shouldApplyAgent := func(key string, isZero bool) bool {
			if agentRaw != nil {
				_, ok := agentRaw[key]
				return !ok
			}
			return isZero
		}

		if shouldApplyAgent("model", a.Model == "") && da.Model != "" {
			a.Model = da.Model
		}
		if shouldApplyAgent("tools", a.Tools == nil) && da.Tools != nil {
			a.Tools = da.Tools
		}
		if shouldApplyAgent("skills", a.Skills == nil) && da.Skills != nil {
			a.Skills = da.Skills
		}
		if shouldApplyAgent("soul", a.Soul == "") && da.Soul != "" {
			a.Soul = da.Soul
		}
		if shouldApplyAgent("memory", a.Memory == nil) && da.Memory != nil {
			a.Memory = da.Memory
		}
		if shouldApplyAgent("prompt", a.Prompt == "") && da.Prompt != "" {
			a.Prompt = da.Prompt
		}
		if shouldApplyAgent("max_iterations", a.MaxIterations == nil) && da.MaxIterations != nil {
			a.MaxIterations = da.MaxIterations
		}
		if shouldApplyAgent("safe_mode", a.SafeMode == nil) && da.SafeMode != nil {
			a.SafeMode = da.SafeMode
		}
	}

	// Additive fields: prepend defaults before step values
	if !d.Env.IsZero() {
		s.Env = s.Env.Prepend(d.Env)
	}
	if d.Preconditions != nil {
		if s.Preconditions == nil {
			s.Preconditions = d.Preconditions
		} else {
			s.Preconditions = combinePreconditions(d.Preconditions, s.Preconditions)
		}
	}
}

// combinePreconditions merges two precondition values into a single []any slice.
// Both values are normalized to arrays and concatenated (first before second).
func combinePreconditions(first, second any) []any {
	normalize := func(v any) []any {
		if arr, ok := v.([]any); ok {
			return arr
		}
		return []any{v}
	}
	a := normalize(first)
	b := normalize(second)
	combined := make([]any, 0, len(a)+len(b))
	combined = append(combined, a...)
	combined = append(combined, b...)
	return combined
}
