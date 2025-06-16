# Installation & Setup

Complete guide to installing and setting up Dagu on various platforms.

## System Requirements

### Minimum Requirements

- **OS**: Linux or macOS
- **Memory**: 128 MB RAM
- **Disk**: 40 MB for binary + space for logs
- **CPU**: Any x86_64 or ARM64 processor

## Installation Methods

### Via Homebrew (macOS/Linux)

The easiest way to install on macOS and Linux:

```bash
# Install
brew install dagu-org/brew/dagu

# Upgrade to latest version
brew upgrade dagu-org/brew/dagu

# Verify installation
dagu version
```

### Via Install Script

Quick installation for all platforms:

```bash
# Download and install latest version
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | bash

# Install specific version
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | bash -s -- v1.14.0

# Install to custom location
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | bash -s -- -d /usr/local/bin
```

### Via Docker

Run Dagu in a container:

```bash
# Basic usage
docker run --rm -it \
  -p 8080:8080 \
  -v ~/.config/dagu:/home/dagu/.config/dagu \
  -v ~/.local/share/dagu:/home/dagu/.local/share/dagu \
  ghcr.io/dagu-org/dagu:latest dagu start-all

# With timezone and authentication
docker run -d \
  --name dagu \
  -p 8080:8080 \
  -v ~/.config/dagu:/home/dagu/.config/dagu \
  -v ~/.local/share/dagu:/home/dagu/.local/share/dagu \
  -e DAGU_TZ="America/New_York" \
  -e DAGU_AUTH_BASIC_USERNAME=admin \
  -e DAGU_AUTH_BASIC_PASSWORD=secret \
  ghcr.io/dagu-org/dagu:latest dagu start-all
```

### Via Docker Compose

For production deployments:

```yaml
# docker-compose.yml
version: '3.8'

services:
  dagu:
    image: ghcr.io/dagu-org/dagu:latest
    container_name: dagu
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      - DAGU_TZ=America/New_York
      - DAGU_AUTH_BASIC_USERNAME=admin
      - DAGU_AUTH_BASIC_PASSWORD=${DAGU_PASSWORD}
    volumes:
      - ./dags:/home/dagu/.config/dagu/dags
      - ./logs:/home/dagu/.local/share/dagu/logs
      - ./data:/home/dagu/.local/share/dagu/data
      - ./config.yaml:/home/dagu/.config/dagu/config.yaml:ro
    command: dagu start-all
```

Start with:
```bash
docker compose up -d
```

### Manual Installation

Download binary from GitHub:

```bash
# Download latest release
VERSION=$(curl -s https://api.github.com/repos/dagu-org/dagu/releases/latest | grep tag_name | cut -d '"' -f 4)
PLATFORM=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# Adjust architecture name
if [ "$ARCH" = "x86_64" ]; then ARCH="amd64"; fi
if [ "$ARCH" = "aarch64" ]; then ARCH="arm64"; fi

# Download binary
curl -L -o dagu "https://github.com/dagu-org/dagu/releases/download/${VERSION}/dagu_${VERSION}_${PLATFORM}_${ARCH}.tar.gz"

# Extract and install
tar -xzf dagu_${VERSION}_${PLATFORM}_${ARCH}.tar.gz
sudo mv dagu /usr/local/bin/
sudo chmod +x /usr/local/bin/dagu

# Verify
dagu version
```

## Directory Structure

Dagu follows the [XDG Base Directory specification](https://specifications.freedesktop.org/basedir-spec/latest/):

```
~/.config/dagu/
├── dags/              # Workflow definitions (YAML files)
├── config.yaml        # Main configuration
└── base.yaml          # Shared base configuration

~/.local/share/dagu/
├── logs/              # Execution logs
│   ├── admin/         # Scheduler/admin logs
│   └── dags/          # Per-DAG execution logs
├── data/              # Workflow state and history
└── suspend/           # Workflow suspend flags
```

### Custom Directory Structure

Use `DAGU_HOME` to organize everything under one directory:

```bash
export DAGU_HOME=/opt/dagu

# Creates:
# /opt/dagu/
# ├── dags/
# ├── logs/
# ├── data/
# ├── suspend/
# ├── config.yaml
# └── base.yaml
```

Or configure individual paths:

```bash
export DAGU_DAGS_DIR=/opt/workflows
export DAGU_LOG_DIR=/var/log/dagu
export DAGU_DATA_DIR=/var/lib/dagu
```

## Initial Setup

### 1. Create Directory Structure

```bash
# Create directories
mkdir -p ~/.config/dagu/dags
mkdir -p ~/.local/share/dagu/{logs,data,suspend}

# Set permissions
chmod 755 ~/.config/dagu
chmod 755 ~/.local/share/dagu
```

### 2. Create Configuration File

```bash
# Create basic config
cat > ~/.config/dagu/config.yaml << EOF
# Server Configuration
host: 127.0.0.1
port: 8080

# UI Customization
ui:
  navbarTitle: "My Workflows"
  navbarColor: "#1976d2"
EOF
```

### 3. Create Your First DAG

```bash
# Create example workflow
cat > ~/.config/dagu/dags/hello.yaml << EOF
name: hello-world
steps:
  - name: greet
    command: echo "Hello from Dagu!"
  - name: date
    command: date
    depends: greet
EOF
```

### 4. Start Dagu

```bash
# Start server and scheduler
dagu start-all

# Or start separately
dagu scheduler &
dagu server
```

### 5. Access the UI

Open http://localhost:8080 in your browser.

## Platform-Specific Setup

### Linux

#### systemd Service

Create `/etc/systemd/system/dagu.service`:

```ini
[Unit]
Description=Dagu Workflow Engine
After=network.target

[Service]
Type=simple
User=dagu
Group=dagu
WorkingDirectory=/opt/dagu
ExecStart=/usr/local/bin/dagu start-all
Restart=always
RestartSec=10

# Security
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/dagu

# Environment
Environment="DAGU_HOME=/opt/dagu"
Environment="DAGU_HOST=0.0.0.0"
Environment="DAGU_PORT=8080"

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl daemon-reload
sudo systemctl enable dagu
sudo systemctl start dagu
sudo systemctl status dagu
```

#### User and Permissions

```bash
# Create dedicated user
sudo useradd -r -s /bin/false -d /opt/dagu dagu

# Create directories
sudo mkdir -p /opt/dagu/{dags,logs,data,suspend}
sudo chown -R dagu:dagu /opt/dagu

# Copy binary
sudo cp dagu /usr/local/bin/
sudo chmod 755 /usr/local/bin/dagu
```

### macOS

#### launchd Service

Create `~/Library/LaunchAgents/org.dagu.agent.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" 
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>org.dagu.agent</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/dagu</string>
        <string>start-all</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/usr/local/var/log/dagu.log</string>
    <key>StandardErrorPath</key>
    <string>/usr/local/var/log/dagu.error.log</string>
</dict>
</plist>
```

Load service:
```bash
launchctl load ~/Library/LaunchAgents/org.dagu.agent.plist
```


## Production Deployment

### Security Checklist

- [ ] Enable authentication (basic auth or API tokens)
- [ ] Use HTTPS with valid certificates
- [ ] Run as non-root user
- [ ] Restrict file permissions
- [ ] Configure firewall rules
- [ ] Set up log rotation
- [ ] Enable monitoring
- [ ] Regular backups


### Log Rotation

Create `/etc/logrotate.d/dagu`:

```
/opt/dagu/logs/**/*.log {
    daily
    rotate 14
    compress
    delaycompress
    missingok
    notifempty
    create 0644 dagu dagu
    sharedscripts
    postrotate
        systemctl reload dagu > /dev/null 2>&1 || true
    endscript
}
```

### Backup Strategy

```bash
#!/bin/bash
# /opt/dagu/scripts/backup.sh

BACKUP_DIR="/backup/dagu/$(date +%Y%m%d)"
mkdir -p "$BACKUP_DIR"

# Backup DAGs
cp -r /opt/dagu/dags "$BACKUP_DIR/"

# Backup data
cp -r /opt/dagu/data "$BACKUP_DIR/"

# Backup configuration
cp /etc/dagu/config.yaml "$BACKUP_DIR/"

# Compress
tar -czf "$BACKUP_DIR.tar.gz" -C /backup/dagu "$(date +%Y%m%d)"
rm -rf "$BACKUP_DIR"

# Keep only last 30 days
find /backup/dagu -name "*.tar.gz" -mtime +30 -delete
```

Add to crontab:
```bash
0 2 * * * /opt/dagu/scripts/backup.sh
```

## Upgrading Dagu

### Using Homebrew

```bash
brew upgrade dagu-org/brew/dagu
```

### Using Install Script

```bash
# Backup current version
sudo cp /usr/local/bin/dagu /usr/local/bin/dagu.backup

# Install new version
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | sudo bash

# Restart service
sudo systemctl restart dagu
```

### Using Docker

```bash
# Pull latest image
docker pull ghcr.io/dagu-org/dagu:latest

# Restart container
docker compose down
docker compose up -d
```

### Manual Upgrade

1. Download new binary
2. Stop service: `sudo systemctl stop dagu`
3. Replace binary: `sudo cp dagu /usr/local/bin/`
4. Start service: `sudo systemctl start dagu`

## Troubleshooting

### Installation Issues

#### Permission Denied

```bash
# Fix binary permissions
chmod +x /usr/local/bin/dagu

# Fix directory permissions
chown -R $USER:$USER ~/.config/dagu
chown -R $USER:$USER ~/.local/share/dagu
```

#### Command Not Found

```bash
# Add to PATH
echo 'export PATH="/usr/local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc

# Or create symlink
sudo ln -s /opt/dagu/bin/dagu /usr/bin/dagu
```

#### Port Already in Use

```bash
# Find process
lsof -i :8080

# Change port
export DAGU_PORT=9000
dagu start-all
```

### Docker Issues

#### Volume Permissions

```bash
# Fix ownership
docker exec dagu chown -R dagu:dagu /home/dagu/.config/dagu
docker exec dagu chown -R dagu:dagu /home/dagu/.local/share/dagu
```

#### Container Exits Immediately

```bash
# Check logs
docker logs dagu

# Run interactively for debugging
docker run --rm -it ghcr.io/dagu-org/dagu:latest /bin/sh
```

## Verification

After installation, verify everything works:

```bash
# Check version
dagu version

# Test server
dagu server &
curl http://localhost:8080/api/v1/dags

# Test scheduler
dagu scheduler &

# Run example DAG
dagu start ~/.config/dagu/dags/hello.yaml
```

## Next Steps

- [Configure the server](/configurations/server) for your environment
- [Set up authentication](/configurations/server#authentication) for security
- [Create your first workflow](/getting-started/first-workflow)
- [Deploy to production](/configurations/operations)