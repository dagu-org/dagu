// Package schema provides JSON schema navigation for agent tools.
package schema

import (
	"encoding/json"
	"fmt"
	"maps"
	"sort"
	"strings"
	"sync"
)

// Registry holds registered JSON schemas for navigation.
type Registry struct {
	mu      sync.RWMutex
	schemas map[string]map[string]any
}

// DefaultRegistry is the global schema registry.
var DefaultRegistry = &Registry{
	schemas: make(map[string]map[string]any),
}

// Register adds a schema to the registry.
func (r *Registry) Register(name string, data []byte) error {
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		return fmt.Errorf("failed to parse schema %s: %w", name, err)
	}
	r.mu.Lock()
	r.schemas[name] = schema
	r.mu.Unlock()
	return nil
}

// Navigate returns formatted schema information for the given path.
func (r *Registry) Navigate(schemaName, path string) (string, error) {
	r.mu.RLock()
	schema, ok := r.schemas[schemaName]
	r.mu.RUnlock()

	if !ok {
		available := r.AvailableSchemas()
		return "", fmt.Errorf("unknown schema: %s (available: %s)", schemaName, strings.Join(available, ", "))
	}

	nav := &navigator{
		root:       schema,
		defs:       getDefinitions(schema),
		path:       path,
		schemaName: schemaName,
		output:     &strings.Builder{},
	}

	return nav.navigate()
}

// AvailableSchemas returns the list of registered schema names in sorted order.
func (r *Registry) AvailableSchemas() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.schemas))
	for name := range r.schemas {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// navigator handles schema path navigation and formatting.
type navigator struct {
	root       map[string]any
	defs       map[string]any
	path       string
	schemaName string
	output     *strings.Builder
}

func (n *navigator) navigate() (string, error) {
	node := n.root

	if n.path != "" {
		parts := strings.Split(n.path, ".")
		var err error
		node, err = n.navigatePath(node, parts)
		if err != nil {
			return "", err
		}
	}

	n.formatNode(node, n.path)

	return n.output.String(), nil
}

func (n *navigator) navigatePath(node map[string]any, parts []string) (map[string]any, error) {
	current := node

	for i, part := range parts {
		current = n.normalizeForNavigation(current)

		props, ok := current["properties"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("path %q: no properties at %q", n.path, strings.Join(parts[:i], "."))
		}

		next, ok := props[part].(map[string]any)
		if !ok {
			available := make([]string, 0, len(props))
			for k := range props {
				available = append(available, k)
			}
			return nil, fmt.Errorf("path %q: field %q not found (available: %s)", n.path, part, strings.Join(available, ", "))
		}

		current = next
	}

	return n.resolveRef(current), nil
}

// maxNormalizationDepth limits recursion when normalizing nested schema constructs.
const maxNormalizationDepth = 10

// normalizeForNavigation resolves refs, oneOf, allOf, and arrays to get to an object with properties.
func (n *navigator) normalizeForNavigation(node map[string]any) map[string]any {
	current := n.resolveRef(node)

	for range maxNormalizationDepth {
		changed := false

		// Handle oneOf/anyOf - find variant with properties
		if found := n.findUnionVariant(current); found != nil {
			current = found
			changed = true
			// Early return if properties found immediately
			if _, hasProps := current["properties"]; hasProps {
				return current
			}
		}

		// Handle allOf - merge schemas
		if allOf, ok := current["allOf"].([]any); ok {
			current = n.mergeAllOf(allOf)
			changed = true
		}

		// Handle array - navigate into items
		if typ, _ := current["type"].(string); typ == "array" {
			if items, ok := current["items"].(map[string]any); ok {
				current = n.resolveRef(items)
				changed = true
			}
		}

		// Resolve any new refs
		if _, hasRef := current["$ref"]; hasRef {
			current = n.resolveRef(current)
			changed = true
		}

		// If we have properties, we're done
		if _, hasProps := current["properties"]; hasProps {
			return current
		}

		if !changed {
			break
		}
	}

	return current
}

// findUnionVariant checks for oneOf/anyOf and returns a navigable variant if found.
func (n *navigator) findUnionVariant(node map[string]any) map[string]any {
	for _, key := range []string{"oneOf", "anyOf"} {
		if union, ok := node[key].([]any); ok {
			if found := n.findNavigableInOneOf(union); found != nil {
				return found
			}
		}
	}
	return nil
}

// findNavigableInOneOf finds a variant that can be navigated (has properties or leads to properties).
func (n *navigator) findNavigableInOneOf(oneOf []any) map[string]any {
	// First pass: look for object with properties (resolves refs automatically)
	for _, opt := range oneOf {
		optMap, ok := opt.(map[string]any)
		if !ok {
			continue
		}
		resolved := n.resolveRef(optMap)
		if _, hasProps := resolved["properties"]; hasProps {
			return resolved
		}
	}

	// Second pass: look for arrays with navigable items
	for _, opt := range oneOf {
		optMap, ok := opt.(map[string]any)
		if !ok {
			continue
		}
		resolved := n.resolveRef(optMap)
		if resolved == nil {
			continue
		}
		if typ, _ := resolved["type"].(string); typ != "array" {
			continue
		}
		items, ok := resolved["items"].(map[string]any)
		if !ok {
			continue
		}
		normalized := n.normalizeForNavigation(n.resolveRef(items))
		if normalized == nil {
			continue
		}
		if _, hasProps := normalized["properties"]; hasProps {
			return normalized
		}
	}

	return nil
}

func (n *navigator) resolveRef(node map[string]any) map[string]any {
	ref, ok := node["$ref"].(string)
	if !ok {
		return node
	}

	// Handle "#/definitions/xxx" format
	if after, ok0 := strings.CutPrefix(ref, "#/definitions/"); ok0 {
		defName := after
		if def, ok := n.defs[defName].(map[string]any); ok {
			return n.resolveRef(def) // Recursively resolve nested refs
		}
	}

	return node
}

func (n *navigator) mergeAllOf(allOf []any) map[string]any {
	merged := make(map[string]any)
	mergedProps := make(map[string]any)

	// Collect all required fields from each allOf item and deduplicate
	requiredSet := make(map[string]struct{})

	for _, item := range allOf {
		if itemMap, ok := item.(map[string]any); ok {
			resolved := n.resolveRef(itemMap)

			// Copy top-level fields (except properties and required which we handle specially)
			for k, v := range resolved {
				if k != "properties" && k != "required" {
					merged[k] = v
				}
			}

			// Merge properties
			if props, ok := resolved["properties"].(map[string]any); ok {
				maps.Copy(mergedProps, props)
			}

			// Collect required fields
			if req, ok := resolved["required"].([]any); ok {
				for _, r := range req {
					if s, ok := r.(string); ok {
						requiredSet[s] = struct{}{}
					}
				}
			}
		}
	}

	if len(mergedProps) > 0 {
		merged["properties"] = mergedProps
	}

	// Deduplicate and set merged required fields
	if len(requiredSet) > 0 {
		allRequired := make([]any, 0, len(requiredSet))
		for r := range requiredSet {
			allRequired = append(allRequired, r)
		}
		// Sort for deterministic output
		sort.Slice(allRequired, func(i, j int) bool {
			return allRequired[i].(string) < allRequired[j].(string)
		})
		merged["required"] = allRequired
	}

	return merged
}

func (n *navigator) formatNode(node map[string]any, path string) {
	// Header
	if path == "" {
		n.output.WriteString(fmt.Sprintf("# %s Schema Root\n\n", capitalizeFirst(n.schemaName)))
	} else {
		n.output.WriteString(fmt.Sprintf("# %s\n\n", path))
	}

	// Resolve ref for display
	node = n.resolveRef(node)

	// Type info
	n.formatType(node)

	// Description
	if desc, ok := node["description"].(string); ok {
		n.output.WriteString(fmt.Sprintf("Description: %s\n\n", desc))
	}

	// Handle oneOf/anyOf - display union type options
	if union := getUnionOptions(node); union != nil {
		n.formatOneOf(union)
		return
	}

	// Handle allOf
	if allOf, ok := node["allOf"].([]any); ok {
		node = n.mergeAllOf(allOf)
	}

	// Properties (direct children)
	if props, ok := node["properties"].(map[string]any); ok {
		n.formatProperties(props, node)
	}

	// Array items
	if typ, _ := node["type"].(string); typ == "array" {
		if items, ok := node["items"].(map[string]any); ok {
			n.output.WriteString("Items:\n")
			items = n.resolveRef(items)
			if itemProps, ok := items["properties"].(map[string]any); ok {
				n.formatProperties(itemProps, items)
			} else {
				n.output.WriteString(fmt.Sprintf("  Type: %s\n", getType(items)))
			}
		}
	}

	// Enum values
	if enum, ok := node["enum"].([]any); ok {
		n.output.WriteString("Allowed values: ")
		vals := make([]string, len(enum))
		for i, v := range enum {
			vals[i] = fmt.Sprintf("%v", v)
		}
		n.output.WriteString(strings.Join(vals, ", "))
		n.output.WriteString("\n")
	}

	// Default value
	if def, ok := node["default"]; ok {
		n.output.WriteString(fmt.Sprintf("Default: %v\n", def))
	}
}

func (n *navigator) formatType(node map[string]any) {
	typ := getType(node)
	if typ != "" {
		n.output.WriteString(fmt.Sprintf("Type: %s\n", typ))
	}
}

func (n *navigator) formatOneOf(options []any) {
	n.output.WriteString("Valid options (oneOf):\n\n")

	for i, opt := range options {
		if optMap, ok := opt.(map[string]any); ok {
			resolved := n.resolveRef(optMap)
			n.output.WriteString(fmt.Sprintf("Option %d: %s\n", i+1, getType(resolved)))
			if desc, ok := resolved["description"].(string); ok {
				n.output.WriteString(fmt.Sprintf("  %s\n", desc))
			}
			if props, ok := resolved["properties"].(map[string]any); ok && len(props) > 0 {
				n.output.WriteString("  Properties:\n")
				for name := range props {
					n.output.WriteString(fmt.Sprintf("    - %s\n", name))
				}
			}
			n.output.WriteString("\n")
		}
	}
}

func (n *navigator) formatProperties(props map[string]any, parent map[string]any) {
	requiredSet := getRequiredSet(parent)

	n.output.WriteString("Properties:\n")

	for name, prop := range props {
		propMap, ok := prop.(map[string]any)
		if !ok {
			continue
		}
		resolved := n.resolveRef(propMap)
		typ := getType(resolved)
		reqStr := ""
		if requiredSet[name] {
			reqStr = ", required"
		}
		desc := truncateDescription(resolved["description"])
		n.output.WriteString(fmt.Sprintf("- %s (%s%s): %s\n", name, typ, reqStr, desc))
	}
}

// getUnionOptions returns the oneOf or anyOf array if present, nil otherwise.
func getUnionOptions(node map[string]any) []any {
	for _, key := range []string{"oneOf", "anyOf"} {
		if union, ok := node[key].([]any); ok {
			return union
		}
	}
	return nil
}

func getDefinitions(schema map[string]any) map[string]any {
	if defs, ok := schema["definitions"].(map[string]any); ok {
		return defs
	}
	return make(map[string]any)
}

func getType(node map[string]any) string {
	if t, ok := node["type"].(string); ok {
		return t
	}
	if _, ok := node["oneOf"]; ok {
		return "oneOf"
	}
	if _, ok := node["anyOf"]; ok {
		return "anyOf"
	}
	if _, ok := node["allOf"]; ok {
		return "allOf"
	}
	if _, ok := node["$ref"]; ok {
		return "ref"
	}
	if _, ok := node["properties"]; ok {
		return "object"
	}
	return "unknown"
}

func getRequiredSet(node map[string]any) map[string]bool {
	req, ok := node["required"].([]any)
	if !ok {
		return nil
	}
	result := make(map[string]bool, len(req))
	for _, r := range req {
		if s, ok := r.(string); ok {
			result[s] = true
		}
	}
	return result
}

func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func truncateDescription(v any) string {
	desc, ok := v.(string)
	if !ok {
		return ""
	}
	const maxLen = 100
	if len(desc) > maxLen {
		return desc[:maxLen-3] + "..."
	}
	return desc
}
