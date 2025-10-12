# Keepalive

A minimal binary that keeps containers running by waiting for termination signals (SIGTERM, SIGINT).

## Purpose

This program is used by Dagu to keep containers alive while DAGs are running. It's implemented in Zig for minimal binary size and cross-platform compatibility.

## Building

To build keepalive binaries for all supported architectures:

```bash
make build-keepalive
```

This will:
1. Use Zig to cross-compile small binaries for multi platform
2. Place binaries in `internal/container/assets/`
3. Generate SHA256 checksums for verification

### Prerequisites

- Zig compiler (install via `brew install zig` on macOS)

## Supported Platforms

- **Darwin (macOS)**: amd64, arm64
- **Linux**: 386, amd64, arm64, armv6, armv7, ppc64le, s390x
- **Windows**: 386, amd64, arm64

Note: BSD targets (FreeBSD, NetBSD, OpenBSD) require additional setup for cross-compilation from macOS.

## Verifying Pre-built Binaries

The pre-built binaries in `internal/container/assets/` come with checksums for verification.

### On Linux/Unix:
```bash
cd internal/container/assets
sha256sum -c keepalive_checksums.txt
```

### On macOS:
```bash
cd internal/container/assets
shasum -a 256 -c keepalive_checksums.txt
```
