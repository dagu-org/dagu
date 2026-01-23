package types

import (
	"fmt"
	"sort"
	"strings"

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
		entry := parseTagEntry(part)
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
		value := stringifyValue(m[key])
		t.entries = append(t.entries, TagEntry{
			key:   strings.TrimSpace(key),
			value: strings.TrimSpace(value),
		})
	}
	return nil
}

func (t *TagsValue) parseArray(arr []any) error {
	for i, item := range arr {
		switch v := item.(type) {
		case map[string]any:
			// Map entry: { key: value }
			// Sort keys for deterministic ordering
			keys := make([]string, 0, len(v))
			for key := range v {
				keys = append(keys, key)
			}
			sort.Strings(keys)

			for _, key := range keys {
				value := stringifyValue(v[key])
				t.entries = append(t.entries, TagEntry{
					key:   strings.TrimSpace(key),
					value: strings.TrimSpace(value),
				})
			}
		case string:
			// String entry: "key=value" or "key"
			entry := parseTagEntry(v)
			if entry.key != "" {
				t.entries = append(t.entries, entry)
			}
		default:
			return fmt.Errorf("tags[%d]: expected map or string, got %T", i, item)
		}
	}
	return nil
}

// parseTagEntry parses a single tag string into TagEntry.
func parseTagEntry(s string) TagEntry {
	s = strings.TrimSpace(s)
	if s == "" {
		return TagEntry{}
	}

	k, v, found := strings.Cut(s, "=")
	k = strings.TrimSpace(k)

	if !found {
		return TagEntry{key: k}
	}

	return TagEntry{
		key:   k,
		value: strings.TrimSpace(v),
	}
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
