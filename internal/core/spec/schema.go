package spec

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/google/jsonschema-go/jsonschema"
)

// resolveSchemaFromParams extracts a schema reference from params and resolves it.
// Returns (nil, nil) if no schema is declared.
func resolveSchemaFromParams(params any, workingDir, dagLocation string) (*jsonschema.Resolved, error) {
	schemaRef := extractSchemaReference(params)
	if schemaRef == "" {
		return nil, nil
	}
	return getSchemaFromRef(workingDir, dagLocation, schemaRef)
}

// Schema Ref can be a local file (relative or absolute paths), or a remote URL
func getSchemaFromRef(workingDir string, dagLocation string, schemaRef string) (*jsonschema.Resolved, error) {
	var schemaData []byte
	var err error

	// Check if it's a URL or file path
	if strings.HasPrefix(schemaRef, "http://") || strings.HasPrefix(schemaRef, "https://") {
		schemaData, err = loadSchemaFromURL(schemaRef)
	} else {
		schemaData, err = loadSchemaFromFile(workingDir, dagLocation, schemaRef)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load schema from %s: %w", schemaRef, err)
	}

	var schema jsonschema.Schema
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		return nil, fmt.Errorf("failed to parse schema JSON: %w", err)
	}

	resolveOptions := &jsonschema.ResolveOptions{
		ValidateDefaults: true,
	}

	resolvedSchema, err := schema.Resolve(resolveOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve schema: %w", err)
	}

	return resolvedSchema, nil
}

// loadSchemaFromURL loads a JSON schema from a URL.
func loadSchemaFromURL(schemaURL string) (data []byte, err error) {
	// Validate URL to prevent potential security issues (and satisfy linter :P)
	parsedURL, err := url.Parse(schemaURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("unsupported URL scheme: %s", parsedURL.Scheme)
	}

	req, err := http.NewRequest("GET", schemaURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	data, err = io.ReadAll(resp.Body)
	return data, err
}

// loadSchemaFromFile loads a JSON schema from a file path.
func loadSchemaFromFile(workingDir string, dagLocation string, filePath string) ([]byte, error) {
	// Try to resolve the schema file path in the following order:
	// 1) Current working directory (default ResolvePath behavior)
	// 2) DAG's workingDir value
	// 3) Directory of the DAG file (where it was loaded from)

	var tried []string

	// Attempts a candidate by joining base and filePath (if base provided),
	// resolving env/tilde + absolute path, checking existence, and reading.
	tryCandidate := func(label, base string) ([]byte, string, error) {
		var candidate string
		if strings.TrimSpace(base) == "" {
			candidate = filePath
		} else {
			candidate = filepath.Join(base, filePath)
		}
		resolved, err := fileutil.ResolvePath(candidate)
		if err != nil {
			tried = append(tried, fmt.Sprintf("%s: resolve error: %v", label, err))
			return nil, "", err
		}
		if !fileutil.FileExists(resolved) {
			tried = append(tried, fmt.Sprintf("%s: %s", label, resolved))
			return nil, resolved, os.ErrNotExist
		}
		data, err := os.ReadFile(resolved) // #nosec G304 - validated path
		if err != nil {
			tried = append(tried, fmt.Sprintf("%s: %s (read error: %v)", label, resolved, err))
			return nil, resolved, err
		}
		return data, resolved, nil
	}

	// 1) As provided (CWD/env/tilde expansion handled by ResolvePath)
	if data, _, err := tryCandidate("cwd", ""); err == nil {
		return data, nil
	}

	// 2) From DAG's workingDir value if present
	if wd := strings.TrimSpace(workingDir); wd != "" {
		if data, _, err := tryCandidate(fmt.Sprintf("workingDir(%s)", wd), wd); err == nil {
			return data, nil
		}
	}

	// 3) From the directory of the DAG file used to build
	if dagLocation != "" {
		base := filepath.Dir(dagLocation)
		if data, _, err := tryCandidate(fmt.Sprintf("dagDir(%s)", base), base); err == nil {
			return data, nil
		}
	}

	if len(tried) == 0 {
		return nil, fmt.Errorf("failed to resolve schema file path: %s (no candidates)", filePath)
	}
	return nil, fmt.Errorf("schema file not found for %q; tried %s", filePath, strings.Join(tried, ", "))
}

// extractSchemaReference extracts the schema reference from a params map.
// Returns the schema reference as a string if present and valid, empty string otherwise.
func extractSchemaReference(params any) string {
	paramsMap, ok := params.(map[string]any)
	if !ok {
		return ""
	}

	schemaRef, hasSchema := paramsMap["schema"]
	if !hasSchema {
		return ""
	}

	schemaRefStr, ok := schemaRef.(string)
	if !ok {
		return ""
	}

	return schemaRefStr
}
