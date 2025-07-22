# Konf-Helper
[![GitHub release](https://img.shields.io/github/release/gerolf-vent/konf-helper.svg)](https://github.com/gerolf-vent/konf-helper/releases)
[![Docker Image](https://img.shields.io/badge/docker-ghcr.io%2Fgerolf--vent%2Fkonf--helper-blue)](https://ghcr.io/gerolf-vent/konf-helper)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

A lightweight Kubernetes sidecar for dynamic configuration management and synchronization.

## Overview

Konf-Helper is a Go-based configuration sidecar designed to run alongside your main application containers in Kubernetes. It watches configuration directories, processes templates with the powerful Sprig template engine, and automatically syncs changes to destination paths while optionally notifying processes of these updates.

## Features

- **Real-time Configuration Sync**: Watches source directories and automatically syncs changes to destination paths
- **Template Processing**: Built-in support for Go templates with [Sprig functions](https://masterminds.github.io/sprig/) for dynamic configuration generation
- **Process Notification**: Send signals to processes when configurations change
- **Debounced Updates**: Configurable debouncing to prevent excessive updates during rapid file changes
- **Kubernetes-Ready**: Built for sidecar patterns with health and readiness endpoints
- **Security-First**: Configurable file permissions, ownership, and path restrictions
- **Observable**: Structured logging with configurable debug levels

## Quick Start

### Basic Usage

```bash
# Sync all files from /etc/config to /app/config
konf-helper /etc/config:*:/app/config

# Process yaml templates and set file permissions
konf-helper /etc/templates:*.yaml:/app/config:1000:1001:644

# Sync all files and notify a process when changes occur
konf-helper --process=nginx /etc/config:*:/app/config
```

### Docker

```bash
docker run ghcr.io/gerolf-vent/konf-helper:latest /etc/config:*:/app/config
```

### Kubernetes Sidecar

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: app
spec:
  shareProcessNamespace: true
  initContainers:
  - name: konf-helper
    image: ghcr.io/gerolf-vent/konf-helper:latest
    restartPolicy: Always
    args:
    - "--process=app"
    - "/etc/source:*.conf:/app/config:1000:1001:644"
    volumeMounts:
    - name: config-src
      mountPath: /etc/source
      readOnly: true
    - name: config
      mountPath: /app/config
    ports:
    - containerPort: 9952
      name: health
    livenessProbe:
      httpGet:
        path: /healthz
        port: 9952
    readinessProbe:
      httpGet:
        path: /readyz
        port: 9952
  containers:
  - name: app
    image: myapp:latest
    volumeMounts:
    - name: config
      mountPath: /app/config
      readOnly: true
  volumes:
  - name: config-src
    configMap:
      name: app-config
  - name: config
    emptyDir: {}
```

## Configuration Syntax

Each path configuration follows the format:
```
srcPath[:fileGlob][:dstPath][:owner][:group][:mode]
```

### Parameters

- **srcPath** (required): Absolute path to source directory
- **fileGlob** (optional): File pattern to match (default: `*`)
- **dstPath** (optional): Absolute path to destination directory
- **owner** (optional): Numeric user ID for file ownership
- **group** (optional): Numeric group ID for file ownership  
- **mode** (optional): Octal file permissions (e.g., `644`)

Optional parameters can be left empty, so e.g. use `/src::/dst::1001` to only set the group.

### Examples

```bash
# Copy all files from source to destination
konf-helper /etc/config:*:/app/config

# Only sync YAML files with specific permissions
konf-helper /etc/templates:*.yaml:/app/config:1000:1001:644

# Watch only, no destination (useful for triggering notifications)
konf-helper /etc/watch-only

# Multiple path configurations
konf-helper \
  /etc/app-config:*.conf:/app/config:1000:1001:644 \
  /etc/secrets:*.key:/app/secrets:1000:1001:600
```

## Template Processing

Konf-helper processes files as Go templates with full [Sprig function support](https://masterminds.github.io/sprig/):

### Template Example

**Source file** (`/etc/templates/app.yaml`):
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .app.name | lower }}
  namespace: {{ .app.namespace | default "default" }}
data:
  config.yaml: |
    server:
      port: {{ env "SERVER_PORT" | default 8080 }}
      host: {{ env "SERVER_HOST" | quote }}
    database:
      url: {{ readFile "/etc/secrets/db-url" | trim }}
```

### Built-in Functions

In addition to all Sprig functions, konf-helper provides:

- **`readFile`**: Read file content<br>
  Only files that are direct descendends of a src path (files which are watched) can be read. All other paths will
  cause the template rendering to fail.
  ```yaml
  secret: {{ readFile "/etc/secrets/api-key" }}
  ```

## Command Line Options

```bash
konf-helper [OPTIONS] PATH_CONFIG [PATH_CONFIG...]

Options:
  --delay duration       Debounce delay for path updates (default: 2s)
  --address string       HTTP health check address (default: ":9952")
  --debug                Enable debug logging
  --process string       Process name to notify on configuration changes
  --signal string        Signal to send to the process (default: "HUP")
```

## Health Endpoints

- **`/healthz`**: Health check endpoint
- **`/readyz`**: Readiness check endpoint

Both endpoints return:
- `200 OK`: Service is healthy/ready
- `503 Service Unavailable`: Service is not ready
- `500 Internal Server Error`: Service is unhealthy

## Process Notification

Konf-helper can notify processes when configurations change by sending Unix signals via command line flags:

```bash
# Notify nginx process with SIGHUP when configs change
konf-helper --process=nginx /etc/nginx:*.conf:/app/nginx

# Notify multiple processes (use multiple instances or custom scripting)
konf-helper --process=apache2 --signal=USR1 /etc/apache:*:/app/apache

# Default signal is HUP if not specified
konf-helper --process=nginx /etc/config:*:/app/config
```

## Architecture

Konf-helper implements a robust file synchronization pattern inspired by Kubernetes ConfigMaps:

1. **Atomic Updates**: Creates new timestamped directories for each update
2. **Symlink Management**: Uses `..data` symlinks for atomic switches
3. **Cleanup**: Automatically removes old configuration versions
4. **Debouncing**: Prevents excessive updates during rapid file changes

## Building

```bash
# Build binary
go build -o konf-helper ./cmd/konf-helper

# Run tests
go test ./...

# Build container image
docker build -t konf-helper .
```

## Use Cases

- **Configuration Hot-Reloading**: Update application configs without restarts
- **Template Processing**: Generate configs from templates with dynamic data
- **Secret Management**: Safely sync and process sensitive configuration files
- **Multi-Source Config**: Combine multiple configuration sources into unified destinations
- **Config Validation**: Process and validate configurations before application consumption

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
