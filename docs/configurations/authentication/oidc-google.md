# Google OIDC Setup

Configure Dagu with Google as OIDC provider.

## Prerequisites

- Google Cloud account or Google Workspace
- Access to Google Cloud Console

## Setup Steps

### 1. Create OAuth 2.0 Client ID

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Select or create a project
3. Navigate to "APIs & Services" > "Credentials"
4. Click "Create Credentials" > "OAuth client ID"
5. Configure OAuth consent screen if prompted:
   - User Type: Internal (for Google Workspace) or External
   - Add required scopes: email, profile, openid
6. Application type: "Web application"
7. Add authorized redirect URI:
   ```
   http://localhost:8080/oidc-callback
   ```
   For production:
   ```
   https://dagu.example.com/oidc-callback
   ```
8. Save and copy the Client ID and Client Secret

### 2. Configure Dagu

#### YAML Configuration

```yaml
# ~/.config/dagu/config.yaml
auth:
  oidc:
    clientId: "123456789012-abcdefghijklmnopqrstuvwxyz012345.apps.googleusercontent.com"
    clientSecret: "GOCSPX-1234567890abcdefghijklmno"
    clientUrl: "http://localhost:8080"
    issuer: "https://accounts.google.com"
    scopes:
      - "openid"
      - "profile"
      - "email"
```

#### Environment Variables

```bash
export DAGU_AUTH_OIDC_CLIENT_ID="123456789012-abcdefghijklmnopqrstuvwxyz012345.apps.googleusercontent.com"
export DAGU_AUTH_OIDC_CLIENT_SECRET="GOCSPX-1234567890abcdefghijklmno"
export DAGU_AUTH_OIDC_CLIENT_URL="http://localhost:8080"
export DAGU_AUTH_OIDC_ISSUER="https://accounts.google.com"
export DAGU_AUTH_OIDC_SCOPES="openid,profile,email"

dagu start-all
```

## Google Workspace Setup

### Domain-Wide Access

For Google Workspace domains:

```yaml
auth:
  oidc:
    clientId: "your-client-id"
    clientSecret: "your-secret"
    clientUrl: "https://dagu.company.com"
    issuer: "https://accounts.google.com"
```

### Specific User Access

```yaml
auth:
  oidc:
    # ... google config ...
    whitelist:
      - "admin@company.com"
      - "devops-team@company.com"
      - "ci-bot@company.com"
```

## Production Configuration

```yaml
# Production with HTTPS
auth:
  oidc:
    clientId: "your-production-client-id"
    clientSecret: "your-production-secret"
    clientUrl: "https://dagu.example.com"
    issuer: "https://accounts.google.com"

# Also enable TLS
tls:
  certFile: "/etc/ssl/dagu.crt"
  keyFile: "/etc/ssl/dagu.key"
```

## Testing

1. Start Dagu:
   ```bash
   dagu start-all
   ```

2. Open browser to http://localhost:8080

3. You should be redirected to Google login

4. After login, redirected back to Dagu

5. Check browser developer tools for cookie named `oidc-token`

## Notes

- Google client IDs look like: `[numeric]-[random].apps.googleusercontent.com`
- Client secrets start with `GOCSPX-` for newer applications
- Google supports wildcard redirect URIs for localhost development
- Session duration is 24 hours
- Google issuer is always `https://accounts.google.com`
