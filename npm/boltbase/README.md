# @dagu-org/boltbase

> A powerful Workflow Orchestration Engine with simple declarative YAML API

[![npm version](https://img.shields.io/npm/v/%40dagu-org%2Fboltbase.svg)](https://www.npmjs.com/package/@dagu-org/boltbase)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)

## Installation

```bash
npm install -g @dagu-org/boltbase
```

Or add to your project:

```bash
npm install @dagu-org/boltbase
```

## Usage

### Command Line

After installation, the `boltbase` command will be available:

```bash
# Start the web UI and scheduler
boltbase start-all

# Run a workflow
boltbase start my-workflow.yaml

# Check workflow status
boltbase status my-workflow.yaml
```

### Programmatic Usage

```javascript
const { execute, getBoltbasePath } = require('@dagu-org/boltbase');

// Get path to the binary
const boltbasePath = getBoltbasePath();

// Execute boltbase commands
const child = execute(['start', 'workflow.yaml']);

// Or use async/await
const { executeAsync } = require('@dagu-org/boltbase');

async function runWorkflow() {
  const result = await executeAsync(['start', 'workflow.yaml']);
  console.log('Exit code:', result.code);
  console.log('Output:', result.stdout);
}
```

## Supported Platforms

This package provides pre-built binaries for:

- **Linux**: x64, arm64, arm (v6/v7), ia32, ppc64le, s390x
- **macOS**: x64 (Intel), arm64 (Apple Silicon)
- **Windows**: x64, ia32, arm64
- **FreeBSD**: x64, arm64, ia32, arm
- **OpenBSD**: x64, arm64

If your platform is not supported, please build from source: https://github.com/dagu-org/dagu#building-from-source

## Features

- **Zero Dependencies** - Single binary, no runtime requirements
- **Declarative YAML** - Define workflows in simple YAML format
- **Web UI** - Beautiful dashboard for monitoring and management
- **Powerful Scheduling** - Cron expressions, dependencies, and complex workflows
- **Language Agnostic** - Run any command, script, or executable

## Documentation

For detailed documentation, visit: https://github.com/dagu-org/dagu

## License

GNU General Public License v3.0
