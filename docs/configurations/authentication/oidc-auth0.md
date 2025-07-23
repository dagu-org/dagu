# Auth0 OIDC Setup

Configure Dagu with Auth0 as OIDC provider.

## Prerequisites

- Auth0 account (free tier works)
- Access to Auth0 Dashboard

## Setup Steps

### 1. Create Application in Auth0

1. Log in to [Auth0 Dashboard](https://manage.auth0.com/)
2. Navigate to Applications > Applications
3. Click "Create Application"
4. Choose:
   - Name: `Dagu` (or your preference)
   - Application Type: `Regular Web Applications`
5. Click Create

### 2. Configure Application

1. Go to Settings tab
2. Note down:
   - **Domain**: `your-tenant.auth0.com`
   - **Client ID**: (shown in Basic Information)
   - **Client Secret**: (shown in Basic Information)
3. Configure Application URIs:
   - **Allowed Callback URLs**: 
     ```
     http://localhost:8080/oidc-callback
     ```
     For production add:
     ```
     https://dagu.example.com/oidc-callback
     ```
   - **Allowed Logout URLs** (optional):
     ```
     http://localhost:8080
     https://dagu.example.com
     ```
4. Save Changes

### 3. Configure Dagu

#### YAML Configuration

```yaml
# ~/.config/dagu/config.yaml
auth:
  oidc:
    clientId: "your-auth0-client-id"
    clientSecret: "your-auth0-client-secret"
    clientUrl: "http://localhost:8080"
    issuer: "https://your-tenant.auth0.com/"
    scopes:
      - "openid"
      - "profile"
      - "email"
```

#### Environment Variables

```bash
export DAGU_AUTH_OIDC_CLIENT_ID="your-auth0-client-id"
export DAGU_AUTH_OIDC_CLIENT_SECRET="your-auth0-client-secret"
export DAGU_AUTH_OIDC_CLIENT_URL="http://localhost:8080"
export DAGU_AUTH_OIDC_ISSUER="https://your-tenant.auth0.com/"
export DAGU_AUTH_OIDC_SCOPES="openid,profile,email"

dagu start-all
```

## User Management

### Create Test Users

1. Go to User Management > Users
2. Click "Create User"
3. Choose connection: `Username-Password-Authentication`
4. Enter email and password
5. Click Create

### Email Whitelist

Restrict access to specific users:

```yaml
auth:
  oidc:
    # ... auth0 config ...
    whitelist:
      - "admin@example.com"
      - "team@example.com"
```

## Advanced Configuration

### Custom Domain

If using Auth0 custom domain:

```yaml
auth:
  oidc:
    issuer: "https://auth.yourdomain.com/"
    # ... rest of config
```

### Additional Scopes

Standard OIDC scopes used by Dagu:

```yaml
auth:
  oidc:
    scopes:
      - "openid"
      - "profile"
      - "email"
```

Note: Dagu does not support refresh tokens. Sessions expire after 24 hours.

### Organizations

For Auth0 Organizations:

1. Enable Organizations in Auth0
2. Create organization
3. Add users to organization
4. Update callback URL to include organization:
   ```
   http://localhost:8080/oidc-callback?organization=ORG_ID
   ```

## Social Connections

### Enable Social Login

1. Go to Authentication > Social
2. Enable desired providers (Google, GitHub, etc.)
3. Configure each provider with their credentials
4. No changes needed in Dagu config

Users can now login with social accounts through Auth0.

## Production Configuration

### Security Settings

1. In Auth0 Dashboard > Settings > Advanced:
   - Enable "OIDC Conformant"
   - Set appropriate token expiration
   - Configure refresh token rotation

2. Production Dagu config:
   ```yaml
   auth:
     oidc:
       clientId: "production-client-id"
       clientSecret: "production-secret"
       clientUrl: "https://dagu.example.com"
       issuer: "https://your-tenant.auth0.com/"
   
   # Enable HTTPS
   tls:
     certFile: "/etc/ssl/dagu.crt"
     keyFile: "/etc/ssl/dagu.key"
   ```

### Rate Limits

Auth0 has rate limits:
- Free tier: 1,000 logins/month
- Paid tiers: Higher limits

Monitor usage in Auth0 Dashboard > Monitoring.

## Testing

1. Start Dagu:
   ```bash
   dagu start-all
   ```

2. Access http://localhost:8080

3. You'll be redirected to Auth0 login

4. Login with test user or social account

5. After successful login, redirected back to Dagu

## Troubleshooting URLs

- Auth0 Dashboard: https://manage.auth0.com/
- OpenID Configuration: https://your-tenant.auth0.com/.well-known/openid-configuration
- Test connection: https://your-tenant.auth0.com/authorize?client_id=YOUR_CLIENT_ID

## Notes

- Issuer URL must include trailing slash
- Auth0 supports standard OIDC discovery
- Free tier sufficient for small teams
- Session duration controlled by Auth0 token settings
- Auth0 Universal Login provides customizable UI