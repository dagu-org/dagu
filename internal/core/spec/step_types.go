// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"strconv"
	"strings"
	gotemplate "text/template"

	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/core/spec/types"
	"github.com/goccy/go-yaml"
	"github.com/google/jsonschema-go/jsonschema"
)

type customStepTypeSpec struct {
	Type        string         `yaml:"type,omitempty"`
	Description string         `yaml:"description,omitempty"`
	InputSchema any            `yaml:"input_schema,omitempty"`
	Template    map[string]any `yaml:"template,omitempty"`
}

type customStepType struct {
	Name        string
	Type        string
	Description string
	InputSchema *jsonschema.Resolved
	Template    map[string]any
}

type customStepTypeRegistry struct {
	entries map[string]*customStepType
}

func (r *customStepTypeRegistry) Lookup(name string) (*customStepType, bool) {
	if r == nil {
		return nil, false
	}
	def, ok := r.entries[strings.TrimSpace(name)]
	return def, ok
}

var customStepTypeNameRegexp = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)

var builtinStepTypeNames = map[string]struct{}{
	"agent":         {},
	"archive":       {},
	"chat":          {},
	"command":       {},
	"container":     {},
	"dag":           {},
	"docker":        {},
	"gha":           {},
	"github-action": {},
	"github_action": {},
	"harness":       {},
	"http":          {},
	"jq":            {},
	"k8s":           {},
	"kubernetes":    {},
	"mail":          {},
	"parallel":      {},
	"postgres":      {},
	"redis":         {},
	"router":        {},
	"s3":            {},
	"sftp":          {},
	"shell":         {},
	"sqlite":        {},
	"ssh":           {},
	"subworkflow":   {},
	"template":      {},
}

var customStepForbiddenCallSiteFields = map[string]struct{}{
	"agent":          {},
	"call":           {},
	"command":        {},
	"container":      {},
	"exec":           {},
	"llm":            {},
	"messages":       {},
	"parallel":       {},
	"params":         {},
	"routes":         {},
	"script":         {},
	"shell":          {},
	"shell_packages": {},
	"value":          {},
	"working_dir":    {},
}

func buildCustomStepTypeRegistry(base, local map[string]customStepTypeSpec) (*customStepTypeRegistry, error) {
	if len(base) == 0 && len(local) == 0 {
		return nil, nil
	}

	registry := &customStepTypeRegistry{
		entries: make(map[string]*customStepType, len(base)+len(local)),
	}

	for name, spec := range base {
		normalizedName := strings.TrimSpace(name)
		if _, exists := registry.entries[normalizedName]; exists {
			return nil, core.NewValidationError(
				fmt.Sprintf("step_types.%s", normalizedName),
				normalizedName,
				fmt.Errorf("duplicate custom step type %q is defined in base config", normalizedName),
			)
		}
		def, err := validateCustomStepTypeSpec(name, spec)
		if err != nil {
			return nil, err
		}
		registry.entries[normalizedName] = def
	}

	for name, spec := range local {
		normalizedName := strings.TrimSpace(name)
		if _, exists := registry.entries[normalizedName]; exists {
			return nil, core.NewValidationError(
				fmt.Sprintf("step_types.%s", normalizedName),
				normalizedName,
				fmt.Errorf("duplicate custom step type %q is defined in both base config and DAG", normalizedName),
			)
		}
		def, err := validateCustomStepTypeSpec(name, spec)
		if err != nil {
			return nil, err
		}
		registry.entries[normalizedName] = def
	}

	return registry, nil
}

func expandedCustomStepExecutorType(targetType string, rendered map[string]any) string {
	targetType = strings.TrimSpace(targetType)
	switch targetType {
	case "":
		return ""
	case "command", "shell":
		// Preserve the implicit command executor for ordinary command/script
		// templates so DAG-level container/ssh/redis/harness inference matches
		// plain command steps. Keep explicit command/shell typing for exec
		// templates because exec is defined as a direct-command form.
		if _, hasExec := rendered["exec"]; !hasExec {
			return ""
		}
		return targetType
	default:
		return targetType
	}
}

func validateCustomStepTypeSpec(name string, spec customStepTypeSpec) (*customStepType, error) {
	name = strings.TrimSpace(name)
	if !customStepTypeNameRegexp.MatchString(name) {
		return nil, core.NewValidationError(
			fmt.Sprintf("step_types.%s", name),
			name,
			fmt.Errorf("custom step type names must match %s", customStepTypeNameRegexp.String()),
		)
	}
	if isBuiltinStepTypeName(name) {
		return nil, core.NewValidationError(
			fmt.Sprintf("step_types.%s", name),
			name,
			fmt.Errorf("custom step type name %q conflicts with a builtin step type", name),
		)
	}

	targetType := strings.TrimSpace(spec.Type)
	if targetType == "" {
		return nil, core.NewValidationError(
			fmt.Sprintf("step_types.%s.type", name),
			spec.Type,
			fmt.Errorf("type is required"),
		)
	}
	if !isBuiltinStepTypeName(targetType) {
		return nil, core.NewValidationError(
			fmt.Sprintf("step_types.%s.type", name),
			spec.Type,
			fmt.Errorf("unknown builtin step type %q", targetType),
		)
	}
	if spec.InputSchema == nil {
		return nil, core.NewValidationError(
			fmt.Sprintf("step_types.%s.input_schema", name),
			nil,
			fmt.Errorf("input_schema is required"),
		)
	}
	if len(spec.Template) == 0 {
		return nil, core.NewValidationError(
			fmt.Sprintf("step_types.%s.template", name),
			spec.Template,
			fmt.Errorf("template is required"),
		)
	}
	if _, exists := spec.Template["type"]; exists {
		return nil, core.NewValidationError(
			fmt.Sprintf("step_types.%s.template.type", name),
			spec.Template["type"],
			fmt.Errorf("template.type is not allowed; use step_types.%s.type instead", name),
		)
	}

	inputSchema, err := resolveCustomStepTypeInputSchema(name, spec.InputSchema)
	if err != nil {
		return nil, err
	}

	return &customStepType{
		Name:        name,
		Type:        targetType,
		Description: strings.TrimSpace(spec.Description),
		InputSchema: inputSchema,
		Template:    cloneMap(spec.Template),
	}, nil
}

func resolveCustomStepTypeInputSchema(name string, schemaDecl any) (*jsonschema.Resolved, error) {
	schemaMap, ok := schemaDecl.(map[string]any)
	if !ok {
		return nil, core.NewValidationError(
			fmt.Sprintf("step_types.%s.input_schema", name),
			schemaDecl,
			fmt.Errorf("input_schema must be an inline JSON Schema object"),
		)
	}
	resolved, err := resolveSchemaDeclaration(schemaMap, "", "")
	if err != nil {
		return nil, core.NewValidationError(
			fmt.Sprintf("step_types.%s.input_schema", name),
			schemaDecl,
			err,
		)
	}
	root := resolved.Schema()
	if root == nil || !schemaDeclaresObject(root) {
		return nil, core.NewValidationError(
			fmt.Sprintf("step_types.%s.input_schema", name),
			schemaDecl,
			fmt.Errorf("input_schema must resolve to an object schema"),
		)
	}
	return resolved, nil
}

func schemaDeclaresObject(root *jsonschema.Schema) bool {
	if root == nil {
		return false
	}
	if root.Type == "object" {
		return true
	}
	return len(root.Types) == 1 && root.Types[0] == "object"
}

func isBuiltinStepTypeName(name string) bool {
	_, ok := builtinStepTypeNames[strings.TrimSpace(name)]
	return ok
}

func validateCustomStepInput(stepTypeName string, schema *jsonschema.Resolved, input map[string]any) (map[string]any, error) {
	working := make(map[string]any, len(input))
	maps.Copy(working, input)
	if err := schema.ApplyDefaults(&working); err != nil {
		return nil, core.NewValidationError(
			"config",
			input,
			fmt.Errorf("failed to apply %q input defaults: %w", stepTypeName, err),
		)
	}
	if err := schema.Validate(working); err != nil {
		if relaxed, ok := customStepInputSchemaAllowingRuntimeExpressions(schema, working); ok {
			if relaxedErr := relaxed.Validate(working); relaxedErr == nil {
				return working, nil
			}
		}
		return nil, core.NewValidationError(
			"config",
			input,
			fmt.Errorf("invalid %q input: %w", stepTypeName, err),
		)
	}
	return working, nil
}

func renderCustomStepTemplate(stepTypeName string, template map[string]any, input map[string]any) (map[string]any, error) {
	rendered, err := renderCustomStepTemplateValue(stepTypeName, template, map[string]any{"input": input})
	if err != nil {
		return nil, err
	}
	result, ok := rendered.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("custom step template for %q must render to an object", stepTypeName)
	}
	return result, nil
}

func renderCustomStepTemplateValue(stepTypeName string, value any, data map[string]any) (any, error) {
	switch typed := value.(type) {
	case nil:
		return nil, nil
	case string:
		return renderCustomStepTemplateString(stepTypeName, typed, data)
	case []any:
		rendered := make([]any, 0, len(typed))
		for _, item := range typed {
			v, err := renderCustomStepTemplateValue(stepTypeName, item, data)
			if err != nil {
				return nil, err
			}
			rendered = append(rendered, v)
		}
		return rendered, nil
	case map[string]any:
		if refPath, ok := typed["$input"].(string); ok && len(typed) == 1 {
			resolved, err := resolveCustomStepInputRef(data["input"], refPath)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve %q template input %q: %w", stepTypeName, refPath, err)
			}
			return resolved, nil
		}
		rendered := make(map[string]any, len(typed))
		for key, item := range typed {
			v, err := renderCustomStepTemplateValue(stepTypeName, item, data)
			if err != nil {
				return nil, err
			}
			rendered[key] = v
		}
		return rendered, nil
	default:
		return typed, nil
	}
}

func renderCustomStepTemplateString(stepTypeName string, text string, data map[string]any) (string, error) {
	tmpl, err := gotemplate.New(stepTypeName).
		Option("missingkey=error").
		Funcs(gotemplate.FuncMap{
			"json": func(v any) (string, error) {
				raw, err := json.Marshal(v)
				if err != nil {
					return "", err
				}
				return string(raw), nil
			},
		}).
		Parse(text)
	if err != nil {
		return "", fmt.Errorf("failed to parse template string: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template string: %w", err)
	}
	return buf.String(), nil
}

func resolveCustomStepInputRef(input any, path string) (any, error) {
	current := input
	for segment := range strings.SplitSeq(strings.TrimSpace(path), ".") {
		if segment == "" {
			return nil, fmt.Errorf("path contains an empty segment")
		}
		switch typed := current.(type) {
		case map[string]any:
			next, ok := typed[segment]
			if !ok {
				return nil, fmt.Errorf("field %q does not exist", segment)
			}
			current = next
		case []any:
			index, err := strconv.Atoi(segment)
			if err != nil {
				return nil, fmt.Errorf("segment %q is not a valid array index", segment)
			}
			if index < 0 || index >= len(typed) {
				return nil, fmt.Errorf("array index %d is out of range", index)
			}
			current = typed[index]
		default:
			return nil, fmt.Errorf("segment %q cannot be resolved from %T", segment, current)
		}
	}
	return current, nil
}

func cloneMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = cloneAny(value)
	}
	return dst
}

func cloneAny(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMap(typed)
	case []any:
		dst := make([]any, len(typed))
		for i, item := range typed {
			dst[i] = cloneAny(item)
		}
		return dst
	default:
		return typed
	}
}

func buildCustomStepFromSpec(
	ctx StepBuildContext,
	callSite *step,
	raw map[string]any,
	defs *defaults,
	customType *customStepType,
	forcedName bool,
) (*core.Step, error) {
	if err := validateCustomStepCallSiteFields(callSite, raw); err != nil {
		return nil, fmt.Errorf("step type %q: %w", customType.Name, err)
	}

	input := map[string]any{}
	if callSite.Config != nil {
		input = cloneMap(callSite.Config)
	}
	validatedInput, err := validateCustomStepInput(customType.Name, customType.InputSchema, input)
	if err != nil {
		return nil, err
	}

	rendered, err := renderCustomStepTemplate(customType.Name, customType.Template, validatedInput)
	if err != nil {
		return nil, core.NewValidationError(
			fmt.Sprintf("step_types.%s.template", customType.Name),
			customType.Template,
			err,
		)
	}
	mergedRaw, err := mergeCustomStepRaw(rendered, callSite, raw, customType, forcedName)
	if err != nil {
		return nil, fmt.Errorf("step type %q: %w", customType.Name, err)
	}

	expandedSpec, err := decodeStep(mergedRaw)
	if err != nil {
		return nil, fmt.Errorf("step type %q: failed to decode expanded template: %w", customType.Name, err)
	}
	applyDefaults(expandedSpec, defs, mergedRaw)
	builtStep, err := buildConcreteStep(ctx, expandedSpec)
	if err != nil {
		return nil, fmt.Errorf("step type %q (resolves to %q): %w", customType.Name, customType.Type, err)
	}
	if builtStep.ExecutorConfig.Metadata == nil {
		builtStep.ExecutorConfig.Metadata = make(map[string]any, 1)
	}
	builtStep.ExecutorConfig.Metadata["custom_type"] = customType.Name
	if customType.Description != "" && builtStep.Description == "" {
		builtStep.Description = customType.Description
	}
	return builtStep, nil
}

func mergeCustomStepRaw(
	rendered map[string]any,
	callSite *step,
	raw map[string]any,
	customType *customStepType,
	forcedName bool,
) (map[string]any, error) {
	merged := cloneMap(rendered)
	if expandedType := expandedCustomStepExecutorType(customType.Type, rendered); expandedType != "" {
		merged["type"] = expandedType
	}

	callSiteRaw, err := customStepCallSiteRaw(callSite, raw, forcedName)
	if err != nil {
		return nil, err
	}
	for key, value := range callSiteRaw {
		switch key {
		case "config", "type":
			continue
		case "env":
			combined, err := mergeCustomStepEnvRaw(merged[key], value)
			if err != nil {
				return nil, core.NewValidationError("env", value, err)
			}
			merged[key] = combined
		case "preconditions":
			if current := merged[key]; current != nil {
				merged[key] = combinePreconditions(current, cloneAny(value))
			} else {
				merged[key] = cloneAny(value)
			}
		default:
			merged[key] = cloneAny(value)
		}
	}

	return merged, nil
}

func customStepCallSiteRaw(callSite *step, raw map[string]any, forcedName bool) (map[string]any, error) {
	if raw != nil {
		cloned := cloneMap(raw)
		if forcedName && callSite != nil {
			cloned["name"] = callSite.Name
		}
		return cloned, nil
	}
	if callSite == nil {
		return nil, nil
	}

	yamlBytes, err := yaml.Marshal(callSite)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal custom step call site: %w", err)
	}

	var decoded map[string]any
	if err := yaml.Unmarshal(yamlBytes, &decoded); err != nil {
		return nil, fmt.Errorf("failed to decode custom step call site: %w", err)
	}
	if forcedName {
		decoded["name"] = callSite.Name
	}
	return decoded, nil
}

func mergeCustomStepEnvRaw(base, override any) (any, error) {
	switch {
	case base == nil:
		return cloneAny(override), nil
	case override == nil:
		return cloneAny(base), nil
	}

	baseEnv, err := decodeViaYAML[types.EnvValue](base)
	if err != nil {
		return nil, fmt.Errorf("invalid template env: %w", err)
	}
	overrideEnv, err := decodeViaYAML[types.EnvValue](override)
	if err != nil {
		return nil, fmt.Errorf("invalid call-site env: %w", err)
	}

	combined := overrideEnv.Prepend(baseEnv)
	return envValueToRaw(combined), nil
}

func envValueToRaw(value types.EnvValue) any {
	entries := value.Entries()
	if len(entries) == 0 {
		return nil
	}

	raw := make([]any, 0, len(entries))
	for _, entry := range entries {
		raw = append(raw, map[string]any{entry.Key: entry.Value})
	}
	return raw
}

func validateCustomStepCallSiteFields(callSite *step, raw map[string]any) error {
	if raw != nil {
		for key := range raw {
			if key == "config" || key == "type" {
				continue
			}
			if _, ok := customStepForbiddenCallSiteFields[key]; ok {
				return core.NewValidationError(key, raw[key], fmt.Errorf("field %q is not allowed when using a custom step type", key))
			}
		}
		return nil
	}

	if callSite == nil {
		return nil
	}
	if callSite.WorkingDir != "" {
		return core.NewValidationError("working_dir", callSite.WorkingDir, fmt.Errorf("field %q is not allowed when using a custom step type", "working_dir"))
	}
	if callSite.Command != nil {
		return core.NewValidationError("command", callSite.Command, fmt.Errorf("field %q is not allowed when using a custom step type", "command"))
	}
	if callSite.Exec != nil {
		return core.NewValidationError("exec", callSite.Exec, fmt.Errorf("field %q is not allowed when using a custom step type", "exec"))
	}
	if !callSite.Shell.IsZero() {
		return core.NewValidationError("shell", callSite.Shell.Value(), fmt.Errorf("field %q is not allowed when using a custom step type", "shell"))
	}
	if len(callSite.ShellPackages) > 0 {
		return core.NewValidationError("shell_packages", callSite.ShellPackages, fmt.Errorf("field %q is not allowed when using a custom step type", "shell_packages"))
	}
	if callSite.Script != "" {
		return core.NewValidationError("script", callSite.Script, fmt.Errorf("field %q is not allowed when using a custom step type", "script"))
	}
	if callSite.Call != "" {
		return core.NewValidationError("call", callSite.Call, fmt.Errorf("field %q is not allowed when using a custom step type", "call"))
	}
	if callSite.Params != nil {
		return core.NewValidationError("params", callSite.Params, fmt.Errorf("field %q is not allowed when using a custom step type", "params"))
	}
	if callSite.Parallel != nil {
		return core.NewValidationError("parallel", callSite.Parallel, fmt.Errorf("field %q is not allowed when using a custom step type", "parallel"))
	}
	if callSite.Container != nil {
		return core.NewValidationError("container", callSite.Container, fmt.Errorf("field %q is not allowed when using a custom step type", "container"))
	}
	if callSite.LLM != nil {
		return core.NewValidationError("llm", callSite.LLM, fmt.Errorf("field %q is not allowed when using a custom step type", "llm"))
	}
	if len(callSite.Messages) > 0 {
		return core.NewValidationError("messages", callSite.Messages, fmt.Errorf("field %q is not allowed when using a custom step type", "messages"))
	}
	if len(callSite.Routes) > 0 {
		return core.NewValidationError("routes", callSite.Routes, fmt.Errorf("field %q is not allowed when using a custom step type", "routes"))
	}
	if strings.TrimSpace(callSite.Value) != "" {
		return core.NewValidationError("value", callSite.Value, fmt.Errorf("field %q is not allowed when using a custom step type", "value"))
	}
	return nil
}
