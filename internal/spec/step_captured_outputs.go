// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import "github.com/dagucloud/dagu/v2/internal/ir"

// deriveCapturedStepOutputs records the names a step publishes outside
// DAGU_OUTPUT_FILE so strict step-output references can be validated against
// them. Declarations authored in YAML take precedence, and a step whose
// published names are only known at run time contributes none.
//
// It is safe to call more than once: previously derived names are recomputed.
func deriveCapturedStepOutputs(result *ir.Step) {
	if result == nil {
		return
	}

	authored := make([]ir.StepOutputDeclaration, 0, len(result.Outputs))
	taken := make(map[string]struct{}, len(result.Outputs))
	for _, output := range result.Outputs {
		if output.Source == ir.StepDeclaredOutputSourceCapture {
			continue
		}
		authored = append(authored, output)
		taken[output.Name] = struct{}{}
	}

	for _, derived := range capturedOutputDeclarations(result) {
		if _, exists := taken[derived.Name]; exists {
			continue
		}
		taken[derived.Name] = struct{}{}
		authored = append(authored, derived)
	}

	if len(authored) == 0 {
		result.Outputs = nil
		return
	}
	result.Outputs = authored
}

// capturedOutputDeclarations returns the names published by the mechanism that
// wins at run time, mirroring the precedence in Node.captureOutput.
func capturedOutputDeclarations(step *ir.Step) []ir.StepOutputDeclaration {
	switch {
	case step.HasStdoutOutputs():
		return capturedNames(stdoutOutputNames(step.StdoutOutputs))
	case step.HasStructuredOutput():
		return capturedNames(sortedKeys(step.StructuredOutput))
	case step.HasOutputSchema():
		return outputSchemaDeclarations(step.OutputSchema)
	case isOutputsWriteStep(step):
		return capturedNames(outputsWriteNames(step.ExecutorConfig.Config))
	}
	return nil
}

// stdoutOutputNames returns the field names a stdout outputs config publishes,
// or nil when the published object is only known after decoding stdout.
func stdoutOutputNames(cfg *ir.StepOutputsConfig) []string {
	switch {
	case len(cfg.Fields) > 0:
		return sortedKeys(cfg.Fields)
	case cfg.Field != "":
		return []string{cfg.Field}
	default:
		return nil
	}
}

// outputsWriteNames returns the keys an outputs.write step publishes.
func outputsWriteNames(config map[string]any) []string {
	values, ok := config["values"].(map[string]any)
	if !ok {
		return nil
	}
	return sortedKeys(values)
}

func isOutputsWriteStep(step *ir.Step) bool {
	return step.ExecutorConfig.Type == ir.ExecutorTypeOutputs
}

// outputSchemaDeclarations reads the top-level property names of an output
// schema. A schema without inline properties, such as one built from `$ref` or
// a composition keyword, publishes names that are only known at run time.
func outputSchemaDeclarations(schema map[string]any) []ir.StepOutputDeclaration {
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return nil
	}

	declarations := make([]ir.StepOutputDeclaration, 0, len(properties))
	for _, name := range sortedKeys(properties) {
		if !declaredOutputNamePattern.MatchString(name) {
			continue
		}
		declarations = append(declarations, ir.StepOutputDeclaration{
			Name:   name,
			Type:   schemaOutputType(properties[name]),
			Source: ir.StepDeclaredOutputSourceCapture,
		})
	}
	return declarations
}

// schemaOutputType maps a schema property to a declared output type. Only a
// string property carries a string value; every other shape is JSON text.
func schemaOutputType(property any) string {
	object, ok := property.(map[string]any)
	if !ok {
		return ir.StepDeclaredOutputTypeJSON
	}
	if object["type"] == "string" {
		return ir.StepDeclaredOutputTypeString
	}
	return ir.StepDeclaredOutputTypeJSON
}

func capturedNames(names []string) []ir.StepOutputDeclaration {
	declarations := make([]ir.StepOutputDeclaration, 0, len(names))
	for _, name := range names {
		if !declaredOutputNamePattern.MatchString(name) {
			continue
		}
		declarations = append(declarations, ir.StepOutputDeclaration{
			Name:   name,
			Source: ir.StepDeclaredOutputSourceCapture,
		})
	}
	return declarations
}
