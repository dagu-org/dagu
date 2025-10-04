# macOS Service

## Install Dagu via Homebrew

```bash
# Install Dagu
brew update && brew install dagu

# Update Dagu
brew update && brew upgrade dagu
```


## Create Config File

Create `~/.config/dagu/config.yaml`:

```yaml
host: 127.0.0.1
port: 8525
```

## Create LaunchAgent

Create `~/Library/LaunchAgents/local.dagu.server.plist`:

```sh
vim ~/Library/LaunchAgents/local.dagu.server.plist
```

Contents:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>local.dagu.server</string>
    <key>ProgramArguments</key>
    <array>
        <string>/opt/homebrew/bin/dagu</string>
        <string>start-all</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/dagu.out.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/dagu.err.log</string>
</dict>
</plist>
```

## Start Service

```bash
# Load and start service
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/local.dagu.server.plist

# Check status
launchctl list | grep dagu

# Stop service
launchctl bootout gui/$(id -u)/local.dagu.server

# Restart service
launchctl kickstart -k gui/$(id -u)/local.dagu.server
```

## Uninstall

```bash
# Stop and unload service
launchctl bootout gui/$(id -u)/local.dagu.server

# Remove plist file
rm ~/Library/LaunchAgents/local.dagu.server.plist
```

## Access

Open http://localhost:8525 in your browser.
