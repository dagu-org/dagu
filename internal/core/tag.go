package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
)

// Tag represents a key-value tag with optional value.
// For backward compatibility, key-only tags have an empty Value.
type Tag struct {
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
}

// String returns the canonical string representation of the tag.
// Format: "key=value" or "key" if value is empty.
func (t Tag) String() string {
	if t.Value == "" {
		return t.Key
	}
	return t.Key + "=" + t.Value
}

// IsZero returns true if the tag is empty.
func (t Tag) IsZero() bool {
	return t.Key == ""
}

// Tag validation constants.
const (
	MaxTagKeyLength   = 63
	MaxTagValueLength = 255
)

// Tag validation patterns.
// Keys: alphanumeric, dash, underscore, dot. Must start with letter/number.
// Values: alphanumeric, dash, underscore, dot, slash. Can be empty.
var (
	validTagKeyPattern   = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)
	validTagValuePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_./-]*$`)
)

// ValidateTag validates a tag's key and value format.
func ValidateTag(t Tag) error {
	if t.Key == "" {
		return errors.New("tag key cannot be empty")
	}
	if len(t.Key) > MaxTagKeyLength {
		return fmt.Errorf("tag key exceeds max length %d", MaxTagKeyLength)
	}
	if !validTagKeyPattern.MatchString(t.Key) {
		return fmt.Errorf("tag key %q contains invalid characters (allowed: a-z, A-Z, 0-9, -, _, .)", t.Key)
	}
	if len(t.Value) > MaxTagValueLength {
		return fmt.Errorf("tag value exceeds max length %d", MaxTagValueLength)
	}
	if t.Value != "" && !validTagValuePattern.MatchString(t.Value) {
		return fmt.Errorf("tag value %q contains invalid characters (allowed: a-z, A-Z, 0-9, -, _, ., /)", t.Value)
	}
	return nil
}

// ValidateTags validates all tags in the collection.
func ValidateTags(tags Tags) error {
	var errs []error
	for _, t := range tags {
		if err := ValidateTag(t); err != nil {
			errs = append(errs, fmt.Errorf("tag %q: %w", t.String(), err))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// ParseTag parses a string into a Tag.
// Supports "key=value" and "key" (key-only) formats.
// Both key and value are normalized to lowercase.
func ParseTag(s string) Tag {
	s = strings.TrimSpace(s)
	if s == "" {
		return Tag{}
	}

	key, value, found := strings.Cut(s, "=")
	key = strings.ToLower(strings.TrimSpace(key))

	if !found {
		return Tag{Key: key}
	}

	return Tag{
		Key:   key,
		Value: strings.ToLower(strings.TrimSpace(value)),
	}
}

// Tags represents a collection of tags.
type Tags []Tag

// NewTags creates a Tags collection from a slice of strings.
func NewTags(strs []string) Tags {
	tags := make(Tags, 0, len(strs))
	for _, s := range strs {
		if t := ParseTag(s); !t.IsZero() {
			tags = append(tags, t)
		}
	}
	return tags
}

// Strings returns the tags as a slice of strings for API compatibility.
func (t Tags) Strings() []string {
	if t == nil {
		return nil
	}
	strs := make([]string, len(t))
	for i, tag := range t {
		strs[i] = tag.String()
	}
	return strs
}

// Keys returns all unique keys in the collection.
func (t Tags) Keys() []string {
	seen := make(map[string]struct{})
	keys := make([]string, 0, len(t))
	for _, tag := range t {
		if _, exists := seen[tag.Key]; !exists {
			seen[tag.Key] = struct{}{}
			keys = append(keys, tag.Key)
		}
	}
	return keys
}

// HasKey checks if any tag has the given key.
func (t Tags) HasKey(key string) bool {
	key = strings.ToLower(key)
	for _, tag := range t {
		if tag.Key == key {
			return true
		}
	}
	return false
}

// Get returns all values for a given key.
func (t Tags) Get(key string) []string {
	key = strings.ToLower(key)
	var values []string
	for _, tag := range t {
		if tag.Key == key {
			values = append(values, tag.Value)
		}
	}
	return values
}

// MarshalJSON serializes Tags as an array of strings for backward compatibility.
func (t Tags) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Strings())
}

// UnmarshalJSON deserializes Tags from an array of strings.
func (t *Tags) UnmarshalJSON(data []byte) error {
	var strs []string
	if err := json.Unmarshal(data, &strs); err != nil {
		return err
	}
	*t = NewTags(strs)
	return nil
}

// TagFilterType represents the type of tag filter.
type TagFilterType int

const (
	// TagFilterTypeKeyOnly matches any tag with the specified key (regardless of value).
	TagFilterTypeKeyOnly TagFilterType = iota
	// TagFilterTypeExact matches tags with exact key=value.
	TagFilterTypeExact
	// TagFilterTypeNegation matches if the key does NOT exist.
	TagFilterTypeNegation
	// TagFilterTypeWildcard matches tags using glob patterns (* and ?).
	TagFilterTypeWildcard
)

// TagFilter represents a parsed filter condition.
type TagFilter struct {
	Type  TagFilterType
	Key   string
	Value string
}

// ParseTagFilter parses a filter string into TagFilter.
// Formats:
//   - "key" - matches any tag with that key (KeyOnly)
//   - "key=value" - matches exact key=value (Exact)
//   - "!key" - matches if key does NOT exist (Negation)
//   - "key*" or "key=value*" - matches using glob patterns (Wildcard)
func ParseTagFilter(s string) TagFilter {
	s = strings.TrimSpace(s)
	if s == "" {
		return TagFilter{}
	}

	// Negation filter
	if strings.HasPrefix(s, "!") {
		return TagFilter{
			Type: TagFilterTypeNegation,
			Key:  strings.ToLower(strings.TrimSpace(strings.TrimPrefix(s, "!"))),
		}
	}

	// Key=value filter
	if key, value, found := strings.Cut(s, "="); found {
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.ToLower(strings.TrimSpace(value))

		// Check for wildcard pattern in key or value
		if containsWildcard(key) || containsWildcard(value) {
			return TagFilter{
				Type:  TagFilterTypeWildcard,
				Key:   key,
				Value: value,
			}
		}

		return TagFilter{
			Type:  TagFilterTypeExact,
			Key:   key,
			Value: value,
		}
	}

	// Key-only filter - check for wildcard
	key := strings.ToLower(s)
	if containsWildcard(key) {
		return TagFilter{
			Type: TagFilterTypeWildcard,
			Key:  key,
		}
	}

	return TagFilter{
		Type: TagFilterTypeKeyOnly,
		Key:  key,
	}
}

// containsWildcard checks if a string contains glob wildcard characters.
func containsWildcard(s string) bool {
	return strings.ContainsAny(s, "*?")
}

// MatchesTags checks if a tag collection matches this filter.
func (f TagFilter) MatchesTags(tags Tags) bool {
	switch f.Type {
	case TagFilterTypeKeyOnly:
		return tags.HasKey(f.Key)

	case TagFilterTypeExact:
		for _, t := range tags {
			if t.Key == f.Key && t.Value == f.Value {
				return true
			}
		}
		return false

	case TagFilterTypeNegation:
		return !tags.HasKey(f.Key)

	case TagFilterTypeWildcard:
		for _, t := range tags {
			if !matchGlob(f.Key, t.Key) {
				continue
			}
			// Key matches; check value if specified
			if f.Value == "" || matchGlob(f.Value, t.Value) {
				return true
			}
		}
		return false

	default:
		return false
	}
}

// matchGlob performs simple glob matching with * and ? wildcards.
// * matches zero or more characters, ? matches exactly one character.
// Returns false if the pattern is invalid.
func matchGlob(pattern, s string) bool {
	matched, err := path.Match(pattern, s)
	if err != nil {
		return false
	}
	return matched
}

// MatchesFilters checks if tags match all filters (AND logic).
func (t Tags) MatchesFilters(filters []TagFilter) bool {
	for _, f := range filters {
		if !f.MatchesTags(t) {
			return false
		}
	}
	return true
}
