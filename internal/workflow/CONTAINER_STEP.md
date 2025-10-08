# Container Step - Docker Container Execution

The `container` step type allows you to execute Docker containers within workflows with full control over the container
lifecycle, resource limits, and timeout management.

## Features

- ✅ Run any Docker image
- ✅ Override command and args
- ✅ Set environment variables with template support
- ✅ Mount volumes
- ✅ Configure network mode
- ✅ Set resource limits (memory, CPU)
- ✅ Automatic JSON parsing of container logs
- ✅ Timeout support with automatic cleanup
- ✅ Pull policy control (always, missing, never)
- ✅ Auto-remove containers after execution
- ✅ Privileged mode support

## Basic Usage

### Simple Container Execution

```json
{
  "type": "container",
  "spec": {
    "name": "hello_world",
    "image": "alpine:latest",
    "command": ["echo"],
    "args": ["Hello from container!"]
  }
}
```

### Run Container with Environment Variables

```json
{
  "type": "container",
  "spec": {
    "name": "env_test",
    "image": "alpine:latest",
    "command": ["sh", "-c"],
    "args": ["echo $MY_VAR"],
    "env": {
      "MY_VAR": "Hello World",
      "API_KEY": "{{secrets.api_key}}"
    }
  }
}
```

### Container with Template Expressions

```json
{
  "type": "container",
  "spec": {
    "name": "deploy",
    "image": "deployer:{{version}}",
    "command": ["./deploy.sh"],
    "args": ["{{environment}}", "{{region}}"],
    "env": {
      "API_URL": "{{api.url}}",
      "TIMEOUT": "{{deploy.timeout}}"
    }
  }
}
```

## Advanced Features

### Volume Mounts

Mount host directories or Docker volumes into containers:

```json
{
  "type": "container",
  "spec": {
    "name": "process_data",
    "image": "data-processor:latest",
    "volumes": [
      {
        "source": "/host/data",
        "target": "/data",
        "read_only": false
      },
      {
        "source": "/host/config.yaml",
        "target": "/app/config.yaml",
        "read_only": true
      }
    ]
  }
}
```

### Network Configuration

Control container networking:

```json
{
  "type": "container",
  "spec": {
    "name": "isolated_build",
    "image": "builder:latest",
    "network": "none",
    "command": ["make", "build"]
  }
}
```

**Network modes:**

- `bridge` (default): Connect to bridge network
- `host`: Use host network stack
- `none`: Disable networking
- Custom network name: Connect to specific Docker network

### Resource Limits

Limit container resources:

```json
{
  "type": "container",
  "spec": {
    "name": "resource_limited",
    "image": "memory-intensive:latest",
    "memory_limit_mb": 512,
    "cpu_shares": 512,
    "timeout": 300
  }
}
```

- `memory_limit_mb`: Hard memory limit in megabytes
- `cpu_shares`: Relative CPU weight (1024 = 100% of one core)
- `timeout`: Maximum execution time in seconds (default: 300)

### Pull Policy

Control when Docker images are pulled:

```json
{
  "type": "container",
  "spec": {
    "name": "latest_image",
    "image": "myapp:latest",
    "pull_policy": "always"
  }
}
```

**Pull policies:**

- `missing` (default): Pull only if image doesn't exist locally
- `always`: Always pull the latest image
- `never`: Never pull, fail if image doesn't exist locally

### Privileged Mode

Run containers with elevated privileges:

```json
{
  "type": "container",
  "spec": {
    "name": "docker_build",
    "image": "docker:latest",
    "privileged": true,
    "command": ["docker", "build", "-t", "myimage", "."]
  }
}
```

**Warning:** Only use privileged mode when absolutely necessary, as it grants the container almost all capabilities of
the host machine.

## JSON Output Parsing

Container logs that are valid JSON are automatically parsed:

```json
{
  "type": "container",
  "spec": {
    "name": "api_call",
    "image": "curlimages/curl:latest",
    "command": ["curl"],
    "args": ["-s", "https://api.example.com/data"]
  }
}
```

Access the parsed JSON via `{{api_call.output}}` in subsequent steps.

## Response Structure

The container step stores a response object in the workflow environment:

```json
{
  "container_id": "abc123...",
  "exit_code": 0,
  "logs": "container output",
  "output": { "parsed": "json" },
  "error": "error message if failed"
}
```

### Accessing Response Data

```json
{
  "type": "http",
  "spec": {
    "name": "send_result",
    "url": "https://api.example.com/results",
    "method": "POST",
    "body": {
      "container_id": "{{my_container.container_id}}",
      "exit_code": "{{my_container.exit_code}}",
      "logs": "{{my_container.logs}}",
      "output": "{{my_container.output}}"
    }
  }
}
```

## Configuration Options

| Field             | Type              | Description                      | Default       |
|-------------------|-------------------|----------------------------------|---------------|
| `name`            | string            | Step name (required)             | -             |
| `image`           | string            | Docker image (required)          | -             |
| `command`         | []string          | Override container command       | image default |
| `args`            | []string          | Override container args          | image default |
| `env`             | map[string]string | Environment variables            | {}            |
| `volumes`         | []VolumeMount     | Volume mounts                    | []            |
| `network`         | string            | Network mode                     | bridge        |
| `timeout`         | int               | Timeout in seconds               | 300           |
| `memory_limit_mb` | int64             | Memory limit in MB               | unlimited     |
| `cpu_shares`      | int64             | CPU shares (relative weight)     | unlimited     |
| `pull_policy`     | string            | Image pull policy                | missing       |
| `auto_remove`     | bool              | Remove container after execution | true          |
| `privileged`      | bool              | Run in privileged mode           | false         |

### VolumeMount Structure

| Field       | Type   | Description                         | Default |
|-------------|--------|-------------------------------------|---------|
| `source`    | string | Host path or volume name (required) | -       |
| `target`    | string | Container path (required)           | -       |
| `read_only` | bool   | Mount as read-only                  | false   |

## Complete Example

Full-featured container execution with all options:

```json
{
  "type": "container",
  "spec": {
    "name": "complex_build",
    "image": "builder:{{version}}",
    "command": ["./build.sh"],
    "args": ["--env", "{{environment}}", "--parallel", "4"],
    "env": {
      "BUILD_ENV": "{{environment}}",
      "API_KEY": "{{secrets.build_key}}",
      "CACHE_DIR": "/cache"
    },
    "volumes": [
      {
        "source": "/host/source",
        "target": "/app/source",
        "read_only": true
      },
      {
        "source": "/host/cache",
        "target": "/cache",
        "read_only": false
      },
      {
        "source": "/host/output",
        "target": "/output",
        "read_only": false
      }
    ],
    "network": "build-network",
    "memory_limit_mb": 2048,
    "cpu_shares": 1024,
    "timeout": 1800,
    "pull_policy": "always",
    "auto_remove": true
  }
}
```

## Use Cases

### 1. Running Tests in Containers

```json
{
  "type": "container",
  "spec": {
    "name": "run_tests",
    "image": "node:18-alpine",
    "command": ["npm", "test"],
    "volumes": [
      {
        "source": "/app/source",
        "target": "/app",
        "read_only": true
      }
    ],
    "network": "none",
    "memory_limit_mb": 512,
    "timeout": 600
  }
}
```

### 2. Building Docker Images

```json
{
  "type": "container",
  "spec": {
    "name": "build_image",
    "image": "docker:latest",
    "privileged": true,
    "command": ["docker", "build"],
    "args": ["-t", "{{image_name}}:{{tag}}", "."],
    "volumes": [
      {
        "source": "/var/run/docker.sock",
        "target": "/var/run/docker.sock"
      },
      {
        "source": "/app/source",
        "target": "/build"
      }
    ]
  }
}
```

### 3. Data Processing

```json
{
  "type": "container",
  "spec": {
    "name": "process_data",
    "image": "python:3.11-slim",
    "command": ["python", "/app/process.py"],
    "args": ["--input", "/data/input.csv", "--output", "/data/output.json"],
    "volumes": [
      {
        "source": "/host/data",
        "target": "/data"
      },
      {
        "source": "/host/scripts/process.py",
        "target": "/app/process.py",
        "read_only": true
      }
    ],
    "env": {
      "PYTHONUNBUFFERED": "1"
    },
    "memory_limit_mb": 1024,
    "timeout": 3600
  }
}
```

### 4. Database Migrations

```json
{
  "type": "container",
  "spec": {
    "name": "run_migrations",
    "image": "migrate/migrate:latest",
    "command": ["migrate"],
    "args": [
      "-path", "/migrations",
      "-database", "{{database_url}}",
      "up"
    ],
    "volumes": [
      {
        "source": "/app/migrations",
        "target": "/migrations",
        "read_only": true
      }
    ],
    "network": "host",
    "timeout": 300
  }
}
```

### 5. Security Scanning

```json
{
  "type": "container",
  "spec": {
    "name": "security_scan",
    "image": "aquasec/trivy:latest",
    "command": ["trivy"],
    "args": ["image", "--format", "json", "{{image_to_scan}}"],
    "network": "host",
    "timeout": 600,
    "pull_policy": "always"
  }
}
```

## Error Handling

Containers that exit with a non-zero code will cause the workflow step to error:

```json
{
  "container_id": "abc123...",
  "exit_code": 1,
  "logs": "error: file not found",
  "error": "container exited with code 1"
}
```

## Timeout Behavior

When a container execution times out:

1. The container is stopped gracefully (10 second grace period)
2. The workflow step returns a timeout error
3. If `auto_remove` is true, the container is automatically removed
4. The step fails and the workflow stops (unless error handling is implemented)

## Best Practices

1. **Always Set Timeouts**: Prevent workflows from hanging indefinitely
2. **Use Resource Limits**: Prevent resource exhaustion on the host
3. **Leverage Auto-Remove**: Keep the Docker environment clean
4. **Use Specific Tags**: Avoid `latest` tag in production for reproducibility
5. **Minimize Privileged Mode**: Only use when absolutely necessary
6. **Isolate Network**: Use `network: "none"` for builds and tests that don't need network access
7. **Mount Read-Only**: Mount source code and configurations as read-only when possible
8. **Parse JSON Output**: Structure container output as JSON for easier consumption
9. **Use Template Expressions**: Parameterize images, commands, and environment variables
10. **Pull Policy**: Use `always` for `latest` tags, `missing` for specific versions

## Security Considerations

1. **Privileged Mode**: Grants almost all host capabilities - use sparingly
2. **Volume Mounts**: Be careful with host path mounts - validate paths
3. **Network Access**: Use `network: "none"` when network is not needed
4. **Resource Limits**: Always set memory and CPU limits to prevent DoS
5. **Image Trust**: Only use trusted images from verified sources
6. **Secrets**: Use template expressions for secrets, don't hardcode
7. **Auto-Remove**: Enable to prevent accumulation of containers
8. **Timeout**: Always set reasonable timeouts

## Prerequisites

- Docker daemon must be running and accessible
- Workflow engine must have permissions to access Docker socket
- Required images must be available (locally or from registry)
- Sufficient resources (CPU, memory, disk) on the host

## Troubleshooting

### Container Not Starting

- Check if image exists: `docker images`
- Check Docker daemon: `docker ps`
- Check pull policy and network connectivity
- Check resource availability on host

### Timeout Issues

- Increase `timeout` value
- Check if container is hanging or actually processing
- Review container logs for issues
- Consider resource limits might be too restrictive

### Volume Mount Failures

- Verify host paths exist and have correct permissions
- Check if paths are absolute (not relative)
- Ensure Docker has permission to access host paths

### Network Issues

- Verify network mode is correct for your use case
- Check if custom network exists: `docker network ls`
- Ensure network policies allow required connectivity
