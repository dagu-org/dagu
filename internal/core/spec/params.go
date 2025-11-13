package spec

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/dagu-org/dagu/internal/common/cmdutil"
	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/google/jsonschema-go/jsonschema"
)

// buildParams builds the parameters for the DAG.
func buildParams(ctx BuildContext, spec *definition, dag *core.DAG) error {
	var (
		paramPairs []paramPair
		envs       []string
	)

	if err := parseParams(ctx, spec.Params, &paramPairs, &envs); err != nil {
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
		if err := parseParams(ctx, ctx.opts.Parameters, &overridePairs, &overrideEnvs); err != nil {
			return err
		}
		// Override the default parameters with the command line parameters
		overrideParams(&paramPairs, overridePairs)
	}

	if len(ctx.opts.ParametersList) > 0 {
		var (
			overridePairs []paramPair
			overrideEnvs  []string
		)
		if err := parseParams(ctx, ctx.opts.ParametersList, &overridePairs, &overrideEnvs); err != nil {
			return err
		}
		// Override the default parameters with the command line parameters
		overrideParams(&paramPairs, overridePairs)
	}

	// Validate the parameters against a resolved schema (if declared)
	if !ctx.opts.Has(BuildFlagSkipSchemaValidation) {
		if resolvedSchema, err := resolveSchemaFromParams(spec.Params, spec.WorkingDir, dag.Location); err != nil {
			return fmt.Errorf("failed to get JSON schema: %w", err)
		} else if resolvedSchema != nil {
			updatedPairs, err := validateParams(paramPairs, resolvedSchema)
			if err != nil {
				return err
			}
			paramPairs = updatedPairs
		}
	}

	for _, paramPair := range paramPairs {
		dag.Params = append(dag.Params, paramPair.String())
	}

	dag.Env = append(dag.Env, envs...)

	return nil
}

func validateParams(paramPairs []paramPair, schema *jsonschema.Resolved) ([]paramPair, error) {
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

// parseParams parses and processes the parameters for the DAG.
func parseParams(ctx BuildContext, value any, params *[]paramPair, envs *[]string) error {
	var paramPairs []paramPair

	paramPairs, err := parseParamValue(ctx, value)
	if err != nil {
		return core.NewValidationError("params", value, fmt.Errorf("%w: %s", ErrInvalidParamValue, err))
	}

	// Accumulated vars for sequential param expansion (e.g., Y=${P1})
	accumulatedVars := make(map[string]string)

	for index, paramPair := range paramPairs {
		if !ctx.opts.Has(BuildFlagNoEval) {
			evaluated, err := evalParamValue(ctx, paramPair.Value, accumulatedVars)
			if err != nil {
				return core.NewValidationError("params", paramPair.Value, fmt.Errorf("%w: %s", ErrInvalidParamValue, err))
			}
			paramPair.Value = evaluated
		}

		*params = append(*params, paramPair)

		paramString := paramPair.String()

		// Store in accumulated vars for next param expansion
		// Positional params: $1, $2, $3, ...
		accumulatedVars[strconv.Itoa(index+1)] = paramString

		if paramPair.Name != "" {
			accumulatedVars[paramPair.Name] = paramPair.Value
		}

		if !ctx.opts.Has(BuildFlagNoEval) && paramPair.Name != "" {
			*envs = append(*envs, paramString)
		}

		if paramPair.Name == "" {
			(*params)[index].Name = strconv.Itoa(index + 1)
		}
	}

	return nil
}

func evalParamValue(ctx BuildContext, raw string, accumulatedVars map[string]string) (string, error) {
	var evalOptions []cmdutil.EvalOption

	if len(accumulatedVars) > 0 {
		evalOptions = append(evalOptions, cmdutil.WithVariables(accumulatedVars))
	}

	if ctx.buildEnv != nil {
		evalOptions = append(evalOptions, cmdutil.WithVariables(ctx.buildEnv))
	}

	return cmdutil.EvalString(ctx.ctx, raw, evalOptions...)
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
		return nil, core.NewValidationError("params", v, fmt.Errorf("%w: %T", ErrInvalidParamValue, v))

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
			// Iterate deterministically to avoid random param order from Go maps.
			keys := make([]string, 0, len(m))
			for name := range m {
				keys = append(keys, name)
			}
			sort.Strings(keys)

			for _, name := range keys {
				value := m[name]
				var valueStr string

				switch v := value.(type) {
				case string:
					valueStr = v

				default:
					valueStr = fmt.Sprintf("%v", v)

				}

				paramPair := paramPair{name, valueStr}
				params = append(params, paramPair)
			}

		default:
			return nil, core.NewValidationError("params", m, fmt.Errorf("%w: %T", ErrInvalidParamValue, m))
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

			if !ctx.opts.Has(BuildFlagNoEval) {
				// Perform backtick command substitution
				backtickRegex := regexp.MustCompile("`[^`]*`")

				var cmdErr error
				value = backtickRegex.ReplaceAllStringFunc(
					value,
					func(match string) string {
						var err error
						cmdStr := strings.Trim(match, "`")
						cmdStr, err = cmdutil.EvalString(ctx.ctx, cmdStr)
						if err != nil {
							cmdErr = err
							// Leave the original command if it fails
							return fmt.Sprintf("`%s`", cmdStr)
						}
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
					return nil, core.NewValidationError("params", value, fmt.Errorf("%w: %s", ErrInvalidParamValue, cmdErr))
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

// resolveSchemaFromParams extracts a schema reference from params and resolves it.
// Returns (nil, nil) if no schema is declared.
func resolveSchemaFromParams(params any, workingDir, dagLocation string) (*jsonschema.Resolved, error) {
	schemaRef := extractSchemaReference(params)
	if schemaRef == "" {
		return nil, nil
	}
	return getSchemaFromRef(workingDir, dagLocation, schemaRef)
}

// buildStepParams parses the params field in the step definition.
// Params are converted to map[string]string and stored in step.Params
func buildStepParams(ctx StepBuildContext, def stepDef, step *core.Step) error {
	if def.Params == nil {
		return nil
	}

	// Parse params using existing parseParamValue function
	paramPairs, err := parseParamValue(ctx.BuildContext, def.Params)
	if err != nil {
		return core.NewValidationError("params", def.Params, err)
	}

	// Convert to map[string]string
	paramsData := make(map[string]string)
	for _, pair := range paramPairs {
		paramsData[pair.Name] = pair.Value
	}

	step.Params = core.NewSimpleParams(paramsData)
	return nil
}
