package spec

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/dagu-org/dagu/internal/common/cmdutil"
	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/dagu-org/dagu/internal/core"
	digraph "github.com/dagu-org/dagu/internal/core"
	"github.com/google/jsonschema-go/jsonschema"
)

// buildParams builds the parameters for the DAG.
func buildParams(ctx BuildContext, spec *definition, dag *core.DAG) error {
	var (
		paramPairs []paramPair
		envs       []string
	)

	if err := parseParams(ctx, spec.Params, &paramPairs, &envs, dag); err != nil {
		return err
	}

	// Create default parameters string in the form of "key=value key=value ..."
	var paramsToJoin []string
	for _, paramPair := range paramPairs {
		paramsToJoin = append(paramsToJoin, paramPair.Escaped())
	}
	dag.DefaultParams = strings.Join(paramsToJoin, " ")

	if ctx.opts.Parameters != "" {
		// Parse the parameters from the command line and override the default parameters
		var (
			overridePairs []paramPair
			overrideEnvs  []string
		)
		if err := parseParams(ctx, ctx.opts.Parameters, &overridePairs, &overrideEnvs, dag); err != nil {
			return err
		}
		// Override the default parameters with the command line parameters
		overrideParams(&paramPairs, overridePairs)
		overrideEnvirons(&envs, overrideEnvs)
	}

	if len(ctx.opts.ParametersList) > 0 {
		var (
			overridePairs []paramPair
			overrideEnvs  []string
		)
		if err := parseParams(ctx, ctx.opts.ParametersList, &overridePairs, &overrideEnvs, dag); err != nil {
			return err
		}
		// Override the default parameters with the command line parameters
		overrideParams(&paramPairs, overridePairs)
		overrideEnvirons(&envs, overrideEnvs)
	}

	// Validate the parameters against the provided schema, if it exists
	schemaRef := extractSchemaReference(spec.Params)
	if schemaRef != "" {
		updatedPairs, err := validateParams(paramPairs, schemaRef)
		if err != nil {
			return err
		}
		paramPairs = updatedPairs
	}

	for _, paramPair := range paramPairs {
		dag.Params = append(dag.Params, paramPair.String())
	}

	dag.Env = append(dag.Env, envs...)

	return nil
}

func validateParams(paramPairs []paramPair, schemaRef string) ([]paramPair, error) {
	schema, err := getSchemaFromRef(schemaRef)
	if err != nil {
		return nil, fmt.Errorf("failed to get JSON schema: %w", err)
	}

	// Convert paramPairs to a map for validation
	paramMap := make(map[string]any)
	for _, pair := range paramPairs {
		// Try to parse as JSON first, fall back to string
		var value any
		if err := json.Unmarshal([]byte(pair.Value), &value); err != nil {
			// If JSON parsing fails, use as string
			value = pair.Value
		}
		paramMap[pair.Name] = value
	}

	// Apply schema defaults to the parameter map
	if err := schema.ApplyDefaults(&paramMap); err != nil {
		return nil, fmt.Errorf("failed to apply schema defaults: %w", err)
	}

	if err := schema.Validate(paramMap); err != nil {
		return nil, fmt.Errorf("parameter validation failed: %w", err)
	}

	// Convert the updated paramMap back to paramPair format
	updatedPairs := make([]paramPair, 0, len(paramMap))
	for name, value := range paramMap {
		var valueStr string
		if str, ok := value.(string); ok {
			valueStr = str
		} else {
			// Convert non-string values to JSON string
			if jsonBytes, err := json.Marshal(value); err == nil {
				valueStr = string(jsonBytes)
			} else {
				valueStr = fmt.Sprintf("%v", value)
			}
		}
		updatedPairs = append(updatedPairs, paramPair{Name: name, Value: valueStr})
	}

	return updatedPairs, nil
}

// Schema Ref can be a local file (relative or absolute paths), or a remote URL
func getSchemaFromRef(schemaRef string) (*jsonschema.Resolved, error) {
	var schemaData []byte
	var err error

	// Check if it's a URL or file path
	if strings.HasPrefix(schemaRef, "http://") || strings.HasPrefix(schemaRef, "https://") {
		schemaData, err = loadSchemaFromURL(schemaRef)
	} else {
		schemaData, err = loadSchemaFromFile(schemaRef)
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
func loadSchemaFromFile(filePath string) ([]byte, error) {
	// Resolve the path (handles relative paths, env vars, tilde expansion)
	resolvedPath, err := fileutil.ResolvePath(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve schema file path: %w", err)
	}

	// #nosec G304 - File path is validated by ResolvePath
	return os.ReadFile(resolvedPath)
}

func overrideParams(paramPairs *[]paramPair, override []paramPair) {
	// Override the default parameters with the command line parameters (and satisfy linter :P)
	pairsIndex := make(map[string]int)
	for i, paramPair := range *paramPairs {
		if paramPair.Name != "" {
			pairsIndex[paramPair.Name] = i
		}
	}
	for i, paramPair := range override {
		if paramPair.Name == "" {
			// For positional parameters
			if i < len(*paramPairs) {
				(*paramPairs)[i] = paramPair
			} else {
				*paramPairs = append(*paramPairs, paramPair)
			}
			continue
		}

		if foundIndex, ok := pairsIndex[paramPair.Name]; ok {
			(*paramPairs)[foundIndex] = paramPair
		} else {
			*paramPairs = append(*paramPairs, paramPair)
		}
	}
}

func overrideEnvirons(envs *[]string, override []string) {
	envsIndex := make(map[string]int)
	for i, env := range *envs {
		envsIndex[env] = i
	}
	for _, env := range override {
		if i, ok := envsIndex[env]; !ok {
			*envs = append(*envs, env)
		} else {
			(*envs)[i] = env
		}
	}
}

// parseParams parses and processes the parameters for the DAG.
func parseParams(ctx BuildContext, value any, params *[]paramPair, envs *[]string, dag *core.DAG) error {
	var paramPairs []paramPair

	paramPairs, err := parseParamValue(ctx, value)
	if err != nil {
		return digraph.WrapError("params", value, fmt.Errorf("%w: %s", ErrInvalidParamValue, err))
	}

	// Accumulated vars for sequential param expansion (e.g., Y=${P1})
	accumulatedVars := make(map[string]string)

	for index, paramPair := range paramPairs {
		if !ctx.opts.NoEval {
			// Use os.Expand with accumulated vars to support ${P1} references
			// Also check buildEnv from env vars (e.g., P2=${A001} where A001 is in env)
			paramPair.Value = os.Expand(paramPair.Value, func(key string) string {
				// Check accumulated params first
				if val, ok := accumulatedVars[key]; ok {
					return val
				}
				// Then check env vars from dag.buildEnv (populated by buildEnvs)
				// This allows params to reference env vars (e.g., P2=${A001} where A001 is in env)
				if dag != nil && ctx.buildEnv != nil {
					if val, ok := ctx.buildEnv[key]; ok {
						return val
					}
				}
				// Fall back to real env
				return os.Getenv(key)
			})
		}

		*params = append(*params, paramPair)

		paramString := paramPair.String()

		// Store in accumulated vars for next param expansion
		// Positional params: $1, $2, $3, ...
		accumulatedVars[strconv.Itoa(index+1)] = paramString

		if !ctx.opts.NoEval && paramPair.Name != "" {
			*envs = append(*envs, paramString)
			// Store named param for next param expansion
			accumulatedVars[paramPair.Name] = paramPair.Value
		}

		if paramPair.Name == "" {
			(*params)[index].Name = strconv.Itoa(index + 1)
		}
	}

	return nil
}

// parseParamValue parses the parameters for the DAG.
func parseParamValue(ctx BuildContext, input any) ([]paramPair, error) {
	switch v := input.(type) {
	case nil:
		return nil, nil

	case string:
		return parseStringParams(ctx, v)

	case []any:
		return parseMapParams(ctx, v)

	case []string:
		return parseListParams(ctx, v)

	// At this point, the schema input can be two cases:
	// 1. a map with a "schema" key and a "values" key
	// e.g. { "schema": "./schema.json", "values": { "batch_size": 10, "environment": "dev" } }
	// 2. a map with no "schema" key
	// e.g. { "batch_size": 10, "environment": "dev" }
	case map[string]any:
		schemaRef := extractSchemaReference(v)
		if schemaRef == "" {
			return parseMapParams(ctx, []any{v})
		}

		values, ok := v["values"]
		if !ok {
			return []paramPair{}, nil // Schema-only mode, no values to validate
		}

		return parseMapParams(ctx, []any{values})
	default:
		return nil, digraph.WrapError("params", v, fmt.Errorf("%w: %T", ErrInvalidParamValue, v))

	}
}

func parseListParams(ctx BuildContext, input []string) ([]paramPair, error) {
	var params []paramPair

	for _, v := range input {
		parsedParams, err := parseStringParams(ctx, v)
		if err != nil {
			return nil, err
		}
		params = append(params, parsedParams...)
	}

	return params, nil
}

func parseMapParams(ctx BuildContext, input []any) ([]paramPair, error) {
	var params []paramPair

	for _, m := range input {
		switch m := m.(type) {
		case string:
			parsedParams, err := parseStringParams(ctx, m)
			if err != nil {
				return nil, err
			}
			params = append(params, parsedParams...)

		case map[string]any:
			for name, value := range m {
				var valueStr string

				switch v := value.(type) {
				case string:
					valueStr = v

				default:
					valueStr = fmt.Sprintf("%v", v)

				}

				if !ctx.opts.NoEval {
					parsed, err := cmdutil.EvalString(ctx.ctx, valueStr)
					if err != nil {
						return nil, digraph.WrapError("params", valueStr, fmt.Errorf("%w: %s", ErrInvalidParamValue, err))
					}
					valueStr = parsed
				}

				paramPair := paramPair{name, valueStr}
				params = append(params, paramPair)
			}

		default:
			return nil, digraph.WrapError("params", m, fmt.Errorf("%w: %T", ErrInvalidParamValue, m))
		}
	}

	return params, nil
}

// paramRegex is a regex to match the parameters in the command.
var paramRegex = regexp.MustCompile(
	`(?:([^\s=]+)=)?("(?:\\"|[^"])*"|` + "`(" + `?:\\"|[^"]*)` + "`" + `|[^"\s]+)`,
)

func parseStringParams(ctx BuildContext, input string) ([]paramPair, error) {
	matches := paramRegex.FindAllStringSubmatch(input, -1)

	var params []paramPair

	for _, match := range matches {
		name := match[1]
		value := match[2]

		if strings.HasPrefix(value, `"`) || strings.HasPrefix(value, "`") {
			if strings.HasPrefix(value, `"`) {
				value = strings.Trim(value, `"`)
				value = strings.ReplaceAll(value, `\"`, `"`)
			}

			if !ctx.opts.NoEval {
				// Perform backtick command substitution
				backtickRegex := regexp.MustCompile("`[^`]*`")

				var cmdErr error
				value = backtickRegex.ReplaceAllStringFunc(
					value,
					func(match string) string {
						cmdStr := strings.Trim(match, "`")
						cmdStr = os.ExpandEnv(cmdStr)
						cmdOut, err := exec.Command("sh", "-c", cmdStr).Output() //nolint:gosec
						if err != nil {
							cmdErr = err
							// Leave the original command if it fails
							return fmt.Sprintf("`%s`", cmdStr)
						}
						return strings.TrimSpace(string(cmdOut))
					},
				)

				if cmdErr != nil {
					return nil, digraph.WrapError("params", value, fmt.Errorf("%w: %s", ErrInvalidParamValue, cmdErr))
				}
			}
		}

		params = append(params, paramPair{name, value})
	}

	return params, nil
}

type paramPair struct {
	Name  string
	Value string
}

func (p paramPair) String() string {
	if p.Name != "" {
		return fmt.Sprintf("%s=%s", p.Name, p.Value)
	}
	return p.Value
}

func (p paramPair) Escaped() string {
	if p.Name != "" {
		return fmt.Sprintf("%s=%q", p.Name, p.Value)
	}
	return fmt.Sprintf("%q", p.Value)
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
