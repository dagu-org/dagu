# TLS/HTTPS Configuration

Enable encrypted connections for Dagu.

## Configuration

### YAML Configuration

```yaml
# ~/.config/dagu/config.yaml
tls:
  certFile: /path/to/server.crt
  keyFile: /path/to/server.key
  caFile: /path/to/ca.crt  # Optional
```

### Environment Variables

```bash
export DAGU_CERT_FILE=/path/to/server.crt
export DAGU_KEY_FILE=/path/to/server.key

dagu start-all
```

## Generate Certificates

### Self-Signed Certificate

```bash
# Generate a self-signed certificate for testing
openssl req -x509 -newkey rsa:4096 -keyout server.key -out server.crt \
  -days 365 -nodes -subj "/CN=localhost"
```

### Production Certificate

```bash
# Generate private key
openssl genrsa -out server.key 4096

# Generate certificate signing request
openssl req -new -key server.key -out server.csr \
  -subj "/C=US/ST=State/L=City/O=Organization/CN=dagu.example.com"

# Submit CSR to your CA and get signed certificate
```

## Usage

### Access HTTPS Server

```bash
# With self-signed certificate
curl -k https://localhost:8080/api/v2/dags

# With CA-signed certificate
curl https://dagu.example.com/api/v2/dags

# CLI with HTTPS
export DAGU_API_URL=https://localhost:8080
dagu status
```

## Multi-Environment Setup

```bash
# Development (HTTP)
dagu start-all

# Production (HTTPS)
export DAGU_CERT_FILE=/etc/ssl/dagu.crt
export DAGU_KEY_FILE=/etc/ssl/dagu.key
dagu start-all
```

## Notes

- Both certFile and keyFile must be provided to enable TLS
- Server will listen on HTTPS when TLS is configured
- Self-signed certificates require clients to skip verification
- Use proper CA-signed certificates in production