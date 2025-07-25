# Design Document: Default DAG Sorting Configuration

## Overview

Users need the ability to configure default sorting behavior for DAG listings in the web UI. Currently, sorting is only applied to the current page view, and users have requested server-side sorting that persists across page navigation with configurable defaults.

As reported in issue #1113, users want to see their most recent DAGs first when using timestamp-based naming conventions. This requires configurable default sorting keys and order that are applied automatically when loading the DAG list.

**Issue:** #1113  
**Feedback-from:** @ghansham

## Current State

- Backend API schema (`/api/v2/api.yaml`) only accepts `sort` parameter with value `"name"`
- The API accepts `sort` and `order` query parameters
- **Backend sorting**: Only supports sorting by `name` field (enforced by API schema enum)
- **Frontend sorting**: Other fields (`status`, `lastRun`, `schedule`, `suspended`) are sorted client-side
- Supported sort orders: `asc`, `desc`
- Default behavior: sort by `name` in `asc` order
- Backend sorting happens at the storage layer before pagination

## Proposed Solution

Add configuration options to specify default sorting behavior for DAG listings. These settings will be applied when no explicit sort parameters are provided in the API request.

**Important**: Since the backend API only supports sorting by `name`, we will allow configuration of any sort field (including `status`, `lastRun`, `schedule`, `suspended`) but:
- When `sortField` is `"name"`: Backend sorting will be used
- When `sortField` is any other value: The frontend will need to handle sorting client-side
- No validation errors will be thrown for non-`name` sort fields to maintain flexibility

### Configuration Structure

```yaml
# config.yaml
ui:
  # Existing UI settings
  logEncodingCharset: "UTF-8"
  navbarColor: ""
  navbarTitle: ""
  maxDashboardPageLimit: 100
  
  # New DAG listing configuration
  dagList:
    sortField: "name"     # Backend only supports: name. Frontend supports: name, status, lastRun, schedule, suspended
    sortOrder: "asc"      # Options: asc, desc
```

### Environment Variables

Following Dagu's convention, these settings can also be configured via environment variables:

- `DAGU_UI_DAG_LIST_SORT_FIELD` - Sets the default sort field
- `DAGU_UI_DAG_LIST_SORT_ORDER` - Sets the default sort order

Environment variables take precedence over config file settings.

### Implementation Details

1. **Config Model Update** (`/internal/config/config.go`):
   ```go
   type UI struct {
       LogEncodingCharset    string
       NavbarColor           string
       NavbarTitle           string
       MaxDashboardPageLimit int
       DAGList               DAGListConfig  // New field
   }

   type DAGListConfig struct {
       SortField string
       SortOrder string
   }
   ```

2. **Config Loader Update** (`/internal/config/loader.go`):
   - Add environment variable bindings:
   ```go
   l.bindEnv("ui.dagList.sortField", "UI_DAG_LIST_SORT_FIELD")
   l.bindEnv("ui.dagList.sortOrder", "UI_DAG_LIST_SORT_ORDER")
   ```

3. **API Handler Update** (`/internal/frontend/api/v2/dags.go`):
   - Modify `ListDAGs` function to check config when query params are nil
   - Apply configured defaults before falling back to hardcoded values
   - For backend compatibility, always use `sort=name` when calling the storage layer
   - Include the configured sort preferences in the API response for frontend use

4. **Configuration Validation**:
   - Validate `sortOrder` is either "asc" or "desc"
   - Accept any string value for `sortField` (no validation to allow flexibility)
   - Fall back to safe defaults if invalid values are provided
   - Log warnings only for invalid sort order

5. **Frontend Handling**:
   - Frontend will read the default sort configuration from the API response
   - Apply client-side sorting for fields other than `name`
   - Maintain current client-side sorting logic for non-name fields

6. **Backward Compatibility**:
   - When no configuration is provided, maintain current behavior (sort by name, ascending)
   - Existing API calls with explicit sort parameters override defaults

## Example Usage

### Configuration Example 1: Sort by most recent runs (via config file)
```yaml
ui:
  dagList:
    sortField: "lastRun"
    sortOrder: "desc"
```

### Configuration Example 2: Sort by status (via environment variables)
```bash
export DAGU_UI_DAG_LIST_SORT_FIELD="status"
export DAGU_UI_DAG_LIST_SORT_ORDER="asc"
dagu start-all
```

### Configuration Example 3: Mixed configuration
```yaml
# config.yaml
ui:
  dagList:
    sortField: "name"
    sortOrder: "asc"
```

```bash
# Override just the sort field via environment variable
export DAGU_UI_DAG_LIST_SORT_FIELD="lastRun"
dagu start-all
# Result: Sort by lastRun in ascending order
```

### API Behavior

1. **No query parameters** - Use configured defaults:
   ```
   GET /api/v2/dags
   # Uses sortField and sortOrder from config
   ```

2. **With query parameters** - Override defaults:
   ```
   GET /api/v2/dags?sort=schedule&order=desc
   # Ignores config, uses provided parameters
   ```

## Implementation Steps

1. Update the `UI` struct in `/internal/config/config.go` to include `DAGList` configuration
2. Add environment variable bindings in `/internal/config/loader.go`
3. Create validation logic for the new configuration fields
4. Update the configuration loader to handle the new nested structure
5. Modify `ListDAGs` in `/internal/frontend/api/v2/dags.go` to use config defaults
6. Update configuration documentation:
   - `/docs/configurations/reference.md`:
     - Add `dagList` configuration under the `ui` section
     - Add `DAGU_UI_DAG_LIST_SORT_FIELD` and `DAGU_UI_DAG_LIST_SORT_ORDER` to environment variables
   - `/docs/configurations/server.md`:
     - Update the UI Customization section to include `dagList` configuration
     - Add environment variables `DAGU_UI_DAG_LIST_SORT_FIELD` and `DAGU_UI_DAG_LIST_SORT_ORDER`
   - `CLAUDE.md` - Add new configuration options to the config.yaml example
7. Add tests for configuration validation and API behavior

## Testing Strategy

1. **Unit Tests** (Update existing tests, no new test files):
   - Update config tests in `/internal/config/config_test.go` for new DAGList field
   - Update API handler tests in `/internal/frontend/api/v2/dags_test.go`
   - Test environment variable override behavior
   - Test backward compatibility

2. **Integration Tests**:
   - Update existing integration tests to verify configuration loading
   - Test API endpoint behavior with various configurations

3. **Manual Testing**:
   - Verify UI displays correctly sorted DAGs
   - Test pagination with different sort configurations
   - Verify query parameter override behavior
   - Test client-side sorting for non-name fields

## Documentation Updates

The following documentation sections need to be updated:

### 1. Configuration Reference (`/docs/configurations/reference.md`)

**UI Section:**
```yaml
# UI
ui:
  navbarColor: "#1976d2"     # Hex or CSS color
  navbarTitle: "Dagu"
  logEncodingCharset: "utf-8"
  maxDashboardPageLimit: 100
  dagList:                   # DAG list default sorting
    sortField: "name"        # Options: name, status, lastRun, schedule, suspended
    sortOrder: "asc"         # Options: asc, desc
```

**Environment Variables Section:**
```markdown
### UI
- `DAGU_UI_NAVBAR_COLOR` - Nav bar color
- `DAGU_UI_NAVBAR_TITLE` - Nav bar title
- `DAGU_UI_LOG_ENCODING_CHARSET` - Log encoding
- `DAGU_UI_MAX_DASHBOARD_PAGE_LIMIT` - Dashboard limit
- `DAGU_UI_DAG_LIST_SORT_FIELD` - Default sort field for DAG list
- `DAGU_UI_DAG_LIST_SORT_ORDER` - Default sort order for DAG list
```

### 2. Server Configuration (`/docs/configurations/server.md`)

**UI Customization Section (around line 238):**
```yaml
ui:
  navbarColor: "#1976d2"
  navbarTitle: "Workflows"
  logEncodingCharset: "utf-8"
  dagList:                      # Default DAG list sorting
    sortField: "lastRun"        # Sort by most recent execution
    sortOrder: "desc"           # Newest first
```

**Environment Variables Section (after line 138):**
```markdown
**UI:**
- `DAGU_UI_DAG_LIST_SORT_FIELD` - Default sort field (name/status/lastRun/schedule/suspended)
- `DAGU_UI_DAG_LIST_SORT_ORDER` - Default sort order (asc/desc)
```

### 3. CLAUDE.md Configuration Example

Update the example configuration in CLAUDE.md to include the new `dagList` options under the `ui` section.

## Migration Considerations

- No database migration required (configuration-only change)
- Existing installations will continue working with default behavior
- Configuration is optional - no action required for users happy with current behavior

## Security Considerations

- No security implications - sorting is applied to already-authorized DAG listings
- Input validation prevents injection through configuration values

## Performance Considerations

- No performance impact - sorting already happens in-memory
- Configuration lookup is negligible overhead
- Future optimization: consider index-based sorting in storage layer

## Alternative Approaches Considered

1. **Client-side configuration**: Rejected because it wouldn't persist across sessions
2. **User preferences in database**: Rejected as overly complex for this use case
3. **Environment variables**: Rejected in favor of consistent config.yaml approach

## Future Enhancements

1. Per-user sorting preferences (requires user management system)
2. Multiple sort fields (e.g., sort by status, then by name)
3. Custom sort field definitions
4. Save last-used sort in browser localStorage as override