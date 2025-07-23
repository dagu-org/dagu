# Basic Authentication

Username and password authentication for Dagu.

## Configuration

### YAML Configuration

```yaml
# ~/.config/dagu/config.yaml
auth:
  basic:
    username: admin
    password: secure-password
```

### Environment Variables

```bash
export DAGU_AUTH_BASIC_USERNAME=admin
export DAGU_AUTH_BASIC_PASSWORD=secure-password

dagu start-all
```

## Usage

### CLI Access

```bash
# Using environment variables
export DAGU_USERNAME=admin
export DAGU_PASSWORD=secure-password
dagu status

# Or use legacy variables
export DAGU_BASICAUTH_USERNAME=admin
export DAGU_BASICAUTH_PASSWORD=secure-password
```

### API Access

```bash
# Basic auth header
curl -u admin:secure-password http://localhost:8080/api/v2/dags

# Or with Authorization header
curl -H "Authorization: Basic $(echo -n admin:secure-password | base64)" \
     http://localhost:8080/api/v2/dags
```

## Notes

- Basic authentication is enabled when both username and password are set
- Empty username or password disables basic authentication
- Credentials are checked on every request