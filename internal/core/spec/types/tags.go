package types

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/goccy/go-yaml"
)

// TagEntry represents a single tag entry with key and optional value.
type TagEntry struct {
	key   string
	value string
}

// Key returns the tag key.
func (e TagEntry) Key() string { return e.key }

// Value returns the tag value.
func (e TagEntry) Value() string { return e.value }

// String returns the canonical string representation.
// Format: "key=value" or "key" if value is empty.
func (e TagEntry) String() string {
	if e.value == "" {
		return e.key
	}
	return e.key + "=" + e.value
}

// TagsValue represents tag configuration that can be specified as:
//   - A space-separated string: "foo=bar zoo=baz"
//   - A map of key-value pairs: { foo: bar, zoo: baz }
//   - An array of strings: ["foo=bar", "simple-tag"]
//   - An array of maps: [{ foo: bar }, { zoo: baz }]
//   - Backward compatible: ["tag1", "tag2"] or "tag1, tag2"
//
// YAML examples:
//
//	tags: "foo=bar zoo=baz"
//
//	tags:
//	  foo: bar
//	  zoo: baz
//
//	tags:
//	  - foo=bar
//	  - simple-tag
//
//	tags:
//	  - production
//	  - critical
type TagsValue struct {
	raw     any        // Original value for error reporting
	isSet   bool       // Whether the field was set in YAML
	entries []TagEntry // Parsed entries in order
}

// UnmarshalYAML implements BytesUnmarshaler for goccy/go-yaml.
func (t *TagsValue) UnmarshalYAML(data []byte) error {
	t.isSet = true

	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("tags unmarshal error: %w", err)
	}
	t.raw = raw

	switch v := raw.(type) {
	case string:
		// Space-separated "key=value" or comma-separated simple tags
		return t.parseString(v)

	case map[string]any:
		// Map of key-value pairs
		return t.parseMap(v)

	case []any:
		// Array of maps or strings
		return t.parseArray(v)

	case nil:
		t.isSet = false
		return nil

	default:
		return fmt.Errorf("tags must be string, map, or array, got %T", v)
	}
}

func (t *TagsValue) parseString(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	// Determine separator based on content:
	// - If contains "=" and spaces but no commas, use space separation (key=value format)
	// - Otherwise use comma separation (backward compatible)
	var parts []string
	if strings.Contains(s, "=") && strings.Contains(s, " ") && !strings.Contains(s, ",") {
		// Space-separated key=value format: "foo=bar zoo=baz"
		parts = strings.Fields(s)
	} else {
		// Comma-separated format (backward compatible): "tag1, tag2" or "tag1,tag2"
		parts = strings.Split(s, ",")
	}

	for _, part := range parts {
		entry, err := parseTagEntry(part)
		if err != nil {
			return err
		}
		if entry.key != "" {
			t.entries = append(t.entries, entry)
		}
	}
	return nil
}

func (t *TagsValue) parseMap(m map[string]any) error {
	// Sort keys for deterministic ordering
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		k := strings.TrimSpace(key)
		v := strings.TrimSpace(stringifyValue(m[key]))

		if err := validateKey(k); err != nil {
			return err
		}
		if err := validateValue(v); err != nil {
			return err
		}

		t.entries = append(t.entries, TagEntry{key: k, value: v})
	}
	return nil
}

func (t *TagsValue) parseArray(arr []any) error {
	for i, item := range arr {
		switch v := item.(type) {
		case map[string]any:
			if err := t.parseArrayMapEntry(i, v); err != nil {
				return err
			}
		case string:
			entry, err := parseTagEntry(v)
			if err != nil {
				return fmt.Errorf("tags[%d]: %w", i, err)
			}
			if entry.key != "" {
				t.entries = append(t.entries, entry)
			}
		default:
			return fmt.Errorf("tags[%d]: expected map or string, got %T", i, item)
		}
	}
	return nil
}

// parseArrayMapEntry parses a map entry within an array (e.g., "- key: value").
func (t *TagsValue) parseArrayMapEntry(index int, m map[string]any) error {
	// Sort keys for deterministic ordering
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		k := strings.TrimSpace(key)
		v := strings.TrimSpace(stringifyValue(m[key]))

		if err := validateKey(k); err != nil {
			return fmt.Errorf("tags[%d]: %w", index, err)
		}
		if err := validateValue(v); err != nil {
			return fmt.Errorf("tags[%d]: %w", index, err)
		}

		t.entries = append(t.entries, TagEntry{key: k, value: v})
	}
	return nil
}

// validateKey validates a tag key and returns an error if invalid.
func validateKey(key string) error {
	if key == "" {
		return fmt.Errorf("tag key cannot be empty")
	}
	if len(key) > core.MaxTagKeyLength {
		return fmt.Errorf("tag key %q exceeds max length %d", key, core.MaxTagKeyLength)
	}
	if !core.ValidTagKeyPattern.MatchString(key) {
		return fmt.Errorf("tag key %q contains invalid characters (allowed: a-z, A-Z, 0-9, -, _, .)", key)
	}
	return nil
}

// validateValue validates a tag value and returns an error if invalid.
func validateValue(value string) error {
	if len(value) > core.MaxTagValueLength {
		return fmt.Errorf("tag value %q exceeds max length %d", value, core.MaxTagValueLength)
	}
	if value != "" && !core.ValidTagValuePattern.MatchString(value) {
		return fmt.Errorf("tag value %q contains invalid characters (allowed: a-z, A-Z, 0-9, -, _, ., /)", value)
	}
	return nil
}

// parseTagEntry parses a single tag string into TagEntry with validation.
func parseTagEntry(s string) (TagEntry, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return TagEntry{}, nil
	}

	key, value, hasValue := strings.Cut(s, "=")
	key = strings.TrimSpace(key)

	if err := validateKey(key); err != nil {
		return TagEntry{}, err
	}

	if hasValue {
		value = strings.TrimSpace(value)
		if err := validateValue(value); err != nil {
			return TagEntry{}, err
		}
		return TagEntry{key: key, value: value}, nil
	}

	return TagEntry{key: key}, nil
}

// IsZero returns true if tags were not set in YAML.
func (t TagsValue) IsZero() bool {
	return !t.isSet
}

// Value returns the original raw value for error reporting.
func (t TagsValue) Value() any {
	return t.raw
}

// Entries returns the parsed tag entries in order.
func (t TagsValue) Entries() []TagEntry {
	return t.entries
}

// IsEmpty returns true if tags were set but contain no entries.
func (t TagsValue) IsEmpty() bool {
	return t.isSet && len(t.entries) == 0
}

// Values returns tag entries as strings for backward compatibility.
// Key-only entries return just the key, key-value entries return "key=value".
func (t TagsValue) Values() []string {
	if t.entries == nil {
		return nil
	}
	values := make([]string, len(t.entries))
	for i, entry := range t.entries {
		values[i] = entry.String()
	}
	return values
}
