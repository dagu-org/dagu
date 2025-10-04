# Linux Systemd Service

## Install Dagu

```bash
# Install to /usr/local/bin (system-wide)
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | \
  bash -s -- --install-dir /usr/local/bin

# Create user and directories
sudo useradd -r -s /bin/false dagu
sudo mkdir -p /var/lib/dagu
sudo chown -R dagu:dagu /var/lib/dagu
```

## Create Service

Create `/etc/systemd/system/dagu.service`:

```ini
[Unit]
Description=Dagu Workflow Engine
After=network.target

[Service]
Type=simple
User=dagu
Group=dagu
WorkingDirectory=/var/lib/dagu
ExecStart=/usr/local/bin/dagu start-all
Restart=always
RestartSec=10

Environment="DAGU_HOST=0.0.0.0"
Environment="DAGU_PORT=8525"
Environment="DAGU_HOME=/var/lib/dagu"

[Install]
WantedBy=multi-user.target
```

## Start Service

```bash
# Reload systemd
sudo systemctl daemon-reload

# Enable auto-start
sudo systemctl enable dagu

# Start service
sudo systemctl start dagu

# Check status
sudo systemctl status dagu

# View logs
sudo journalctl -u dagu -f
```

## Access

Open http://your-server:8525 in your browser.
