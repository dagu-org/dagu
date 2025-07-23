# Token Authentication

API token-based authentication for programmatic access.

## Configuration

### YAML Configuration

```yaml
# ~/.config/dagu/config.yaml
auth:
  token:
    value: your-api-token
```

### Environment Variables

```bash
export DAGU_AUTH_TOKEN=your-api-token

dagu start-all
```

## Usage

### CLI Access

```bash
# Using environment variable
export DAGU_API_TOKEN=your-api-token
dagu status

# Or use legacy variable
export DAGU_AUTHTOKEN=your-api-token
```

### API Access

```bash
# Bearer token in Authorization header
curl -H "Authorization: Bearer your-api-token" \
     http://localhost:8080/api/v2/dags

# Works with any HTTP client
wget --header="Authorization: Bearer your-api-token" \
     http://localhost:8080/api/v2/dags
```

## CI/CD Integration

### GitHub Actions

```yaml
- name: Deploy workflow
  env:
    DAGU_API_TOKEN: ${{ secrets.DAGU_TOKEN }}
  run: |
    dagu push workflow.yaml
```

### GitLab CI

```yaml
deploy:
  script:
    - export DAGU_API_TOKEN=$DAGU_TOKEN
    - dagu push workflow.yaml
```

## Notes

- Token authentication is enabled when a token value is set
- Tokens should be treated as secrets
- No expiration mechanism - rotate tokens manually
- Can be used alongside basic authentication