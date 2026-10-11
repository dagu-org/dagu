// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package computer

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/dagucloud/dagu/v2/internal/cmn/workbook"
	"github.com/dagucloud/dagu/v2/internal/desktop"
)

// maxExtractOutline bounds the elements sent to the model beside the
// screenshot, so a window with a huge tree does not overrun the request.
const maxExtractOutline = 400

// extractMiss is the failure of an extract that may not call the model: a
// field it must read has no binding, or the binding no longer fits the
// screen. The binding is kept for a run that lets the model repair it.
type extractMiss struct {
	err error
	via string
}

func (m extractMiss) Error() string { return "the extract cannot run without AI: " + m.err.Error() }
func (m extractMiss) Unwrap() error { return m.err }

// bindableKind returns the column type a scalar schema property binds as,
// and whether it can bind at all. Objects and arrays never bind, and a
// boolean reads a checkbox's state, which follows with tables.
func bindableKind(property any) (workbook.ColumnType, bool) {
	prop, ok := property.(map[string]any)
	if !ok {
		return "", false
	}
	switch prop["type"] {
	case "string":
		return workbook.TypeString, true
	case "number":
		return workbook.TypeNumber, true
	case "integer":
		return workbook.TypeInteger, true
	default:
		return "", false
	}
}

// schemaProperties returns the top-level properties of an extract schema.
func schemaProperties(schema map[string]any) map[string]any {
	properties, _ := schema["properties"].(map[string]any)
	return properties
}

// bindableFields names the schema properties an extract can read from an
// element, with the type each converts to, in sorted order.
func bindableFields(schema map[string]any) []string {
	var names []string
	for name, property := range schemaProperties(schema) {
		if _, ok := bindableKind(property); ok {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

// unbindableFields names the schema properties an extract cannot read
// without the model: objects, arrays, and, until tables land, booleans.
func unbindableFields(schema map[string]any) []string {
	var names []string
	for name, property := range schemaProperties(schema) {
		if _, ok := bindableKind(property); !ok {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

// readingSchema turns an extract's schema into the one the model answers
// when an extract binds: each bindable field becomes an object giving the
// value and the id of the element it was read from, so the run can bind it;
// every other field is passed through and read by the model alone.
func readingSchema(schema map[string]any) map[string]any {
	properties := schemaProperties(schema)
	reading := make(map[string]any, len(properties))
	for name, property := range properties {
		prop, _ := property.(map[string]any)
		if _, ok := bindableKind(property); ok {
			reading[name] = map[string]any{
				"type": "object",
				"properties": map[string]any{
					"value":   map[string]any{"type": prop["type"]},
					"element": map[string]any{"type": "string", "description": "the id of the element the value was read from, or \"\" if read from the screen"},
				},
				"required": []any{"value", "element"},
			}
			continue
		}
		reading[name] = property
	}
	out := maps.Clone(schema)
	out["properties"] = reading
	return out
}

// extractInstruction appends the window's elements to the instruction, so
// the model names the element each value came from, which tells a value it
// read from an element from one it inferred. An outline the host cannot
// read leaves the instruction as it is, and the extract reads by the model
// alone.
func (r *run) extractInstruction(instruction string, front desktop.Element, outline []desktop.Element) string {
	var lines []string
	for _, e := range outline {
		lines = append(lines, fmt.Sprintf("e%d\t%s\t%s\t%s", len(lines)+1, e.Role, e.Name, elementText(e)))
		if len(lines) >= maxExtractOutline {
			break
		}
	}
	if len(lines) == 0 {
		return instruction
	}
	return instruction + "\n\nThe window in front is " + front.Name +
		". Its elements are listed below as id, role, name, value, one per line. " +
		"For each field, set element to the id you read the value from, or \"\" if you read it from the screen.\n" +
		strings.Join(lines, "\n")
}

// elementText is the text an element shows: its value, or its name when it
// has no value, such as a static label.
func elementText(e desktop.Element) string {
	if e.Value != "" {
		return e.Value
	}
	return e.Name
}

// convertField converts text to a field's type, as a later run reads it.
func convertField(value any, kind workbook.ColumnType) (any, error) {
	return workbook.CoerceValue(value, kind)
}

// sameValue reports whether an element's text and the model's value are the
// same once converted to the field's type. Strings compare as a person
// reads them; numbers and integers compare by value.
func sameValue(elemText string, modelValue any, kind workbook.ColumnType) bool {
	ev, err := convertField(elemText, kind)
	if err != nil {
		return false
	}
	mv, err := convertField(modelValue, kind)
	if err != nil {
		return false
	}
	if kind == workbook.TypeString {
		es, _ := ev.(string)
		ms, _ := mv.(string)
		return desktop.NormalizeText(es) == desktop.NormalizeText(ms)
	}
	return ev == mv
}

// elementIndex returns the outline position a model source id names, such
// as "e3", or -1 when the id is empty, malformed, or out of range.
func elementIndex(source string, n int) int {
	num, err := strconv.Atoi(strings.TrimPrefix(source, "e"))
	if err != nil || num < 1 || num > n {
		return -1
	}
	return num - 1
}

// bindField binds a field to the element the model named, when that
// element's value is the field's value and no other element in the window
// gives that value, so the binding is both the element the model read and
// an unambiguous one. front names the application to record.
func (r *run) bindField(name string, kind workbook.ColumnType, modelValue any, named desktop.Element, outline []desktop.Element, front desktop.Element) (fieldBinding, any, bool) {
	if !sameValue(elementText(named), modelValue, kind) {
		return fieldBinding{}, nil, false
	}
	count := 0
	for i := range outline {
		if sameValue(elementText(outline[i]), modelValue, kind) {
			count++
		}
	}
	if count != 1 {
		return fieldBinding{}, nil, false
	}
	rec := r.recordElement(named, nil, front.App)
	if rec == nil {
		return fieldBinding{}, nil, false
	}
	// The bound run publishes the element's converted value, so it reads
	// the same as every later run that reads the binding.
	value, err := convertField(elementText(named), kind)
	if err != nil {
		return fieldBinding{}, nil, false
	}
	return fieldBinding{Name: name, Kind: string(kind), Element: *rec}, value, true
}

// unwrapAnswer separates a model answer into the values to publish and, for
// each bindable field, the value the model read, so the run can bind it.
// When the request used the reading schema, a bindable field's answer is
// {value, element}; otherwise every field is its own value. schema is the
// original extract schema, which says which fields are bindable.
func unwrapAnswer(values map[string]any, schema map[string]any, wrapped bool) (out map[string]any, scalars map[string]any, sources map[string]string) {
	out = make(map[string]any, len(values))
	scalars = map[string]any{}
	sources = map[string]string{}
	properties := schemaProperties(schema)
	for name, value := range values {
		// A bindable field's answer is {value, element} under the reading
		// schema; a model that answered the plain value anyway still
		// publishes, just without a source to bind.
		if inner, ok := value.(map[string]any); wrapped && ok && hasBindableAnswer(properties[name], inner) {
			out[name] = inner["value"]
			scalars[name] = inner["value"]
			source, _ := inner["element"].(string)
			sources[name] = source
			continue
		}
		out[name] = value
	}
	return out, scalars, sources
}

// hasBindableAnswer reports whether a bindable field's answer is the
// {value, element} object the reading schema asked for.
func hasBindableAnswer(property any, answer map[string]any) bool {
	if _, bindable := bindableKind(property); !bindable {
		return false
	}
	_, hasValue := answer["value"]
	_, hasElement := answer["element"]
	return hasValue && hasElement
}

// publishExtract writes an extract's values to the step outputs, keeping
// only the fields the schema lists, as the output names known when the DAG
// loaded.
func (r *run) publishExtract(schema map[string]any, values map[string]any) {
	properties, listed := schema["properties"].(map[string]any)
	for name, value := range values {
		if _, ok := properties[name]; listed && !ok {
			continue
		}
		r.outputs[name] = value
	}
}

// bindExtract binds every bindable field the model read from an element,
// over the window's outline. A field the model read from the screen, with
// no element named, does not bind.
func (r *run) bindExtract(scalars map[string]any, sources map[string]string, schema map[string]any, outline []desktop.Element, front desktop.Element) ([]fieldBinding, map[string]any) {
	var fields []fieldBinding
	values := map[string]any{}
	properties := schemaProperties(schema)
	for _, name := range bindableFields(schema) {
		value, ok := scalars[name]
		if !ok {
			continue
		}
		index := elementIndex(sources[name], len(outline))
		if index < 0 {
			continue
		}
		kind, _ := bindableKind(properties[name])
		if binding, bound, ok := r.bindField(name, kind, value, outline[index], outline, front); ok {
			fields = append(fields, binding)
			values[name] = bound
		}
	}
	return fields, values
}

// readBinding reads every bound field from the screen now, converting each
// to its type. It returns the values, or the reason a field could not be
// read, looking again until findWithin passes.
func (r *run) readBinding(ctx context.Context, index int, els desktop.Elements, entry recording, findWithin time.Duration) (map[string]any, string, error) {
	values := make(map[string]any, len(entry.Fields))
	for _, field := range entry.Fields {
		found, reason, err := r.resolveElement(ctx, index, els, field.Element, findWithin)
		if err != nil {
			return nil, "", err
		}
		if reason != "" {
			return nil, fmt.Sprintf("field %q was not found: %s", field.Name, reason), nil
		}
		kind, err := workbook.ParseColumnType(field.Kind)
		if err != nil {
			return nil, fmt.Sprintf("field %q has an unknown type %q", field.Name, field.Kind), nil
		}
		value, err := convertField(elementText(found), kind)
		if err != nil {
			return nil, fmt.Sprintf("field %q reads %q, which is not %s", field.Name, elementText(found), field.Kind), nil
		}
		values[field.Name] = value
	}
	return values, "", nil
}

// boundNames lists the fields a recording binds, sorted.
func boundNames(entry recording) []string {
	names := make([]string, 0, len(entry.Fields))
	for _, field := range entry.Fields {
		names = append(names, field.Name)
	}
	slices.Sort(names)
	return names
}

// missingBindings names the bindable fields a recording does not bind, so a
// never extract can say what it cannot read.
func missingBindings(schema map[string]any, entry recording) []string {
	bound := map[string]bool{}
	for _, field := range entry.Fields {
		bound[field.Name] = true
	}
	var missing []string
	for _, name := range bindableFields(schema) {
		if !bound[name] {
			missing = append(missing, name)
		}
	}
	return missing
}
