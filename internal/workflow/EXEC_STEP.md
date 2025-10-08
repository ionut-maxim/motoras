# Exec Step - Shell Command Execution with Isolation

The `exec` step type allows you to execute shell commands and scripts within workflows with optional process and network
isolation.

## Features

- ✅ Execute commands directly or through a shell
- ✅ Capture stdout and stderr
- ✅ Automatic JSON parsing of output
- ✅ Template expression support
- ✅ Custom environment variables
- ✅ Working directory configuration
- ✅ Timeout support
- ✅ **Process isolation** (Linux only)
- ✅ **Network isolation** (Linux only)
- ✅ **Resource limits** (Linux only)

## Basic Usage

### Simple Command

```json
{
  "type": "exec",
  "spec": {
    "name": "list_files",
    "command": "ls",
    "args": ["-la", "/tmp"]
  }
}
```

### Shell Command

```json
{
  "type": "exec",
  "spec": {
    "name": "process_data",
    "command": "cat data.txt | grep 'error' | wc -l",
    "shell": true
  }
}
```

### JSON Output Parsing

Commands that output valid JSON will have their output automatically parsed:

```json
{
  "type": "exec",
  "spec": {
    "name": "get_config",
    "command": "cat",
    "args": ["config.json"]
  }
}
```

Access the parsed JSON via `{{get_config.output}}` in subsequent steps.

### With Template Expressions

```json
{
  "type": "exec",
  "spec": {
    "name": "deploy",
    "command": "./deploy.sh",
    "args": ["{{environment}}", "{{version}}"],
    "env": {
      "API_KEY": "{{secrets.api_key}}",
      "REGION": "us-east-1"
    },
    "working_dir": "/app/scripts",
    "timeout": 300
  }
}
```

## Process and Network Isolation

**Note:** Isolation features are only supported on Linux and require appropriate permissions (typically root or
CAP_SYS_ADMIN capability).

### Network Isolation

Disable all network access for the command:

```json
{
  "type": "exec",
  "spec": {
    "name": "isolated_build",
    "command": "./build.sh",
    "shell": true,
    "isolation": {
      "disable_network": true
    }
  }
}
```

This creates a new network namespace with no network interfaces, preventing any network access.

### Process Isolation

Run the command in isolated namespaces:

```json
{
  "type": "exec",
  "spec": {
    "name": "isolated_script",
    "command": "./untrusted-script.sh",
    "shell": true,
    "isolation": {
      "new_pid_namespace": true,
      "new_ipc_namespace": true,
      "new_uts_namespace": true
    }
  }
}
```

**Namespace Isolation Options:**

- `new_pid_namespace`: Process sees itself as PID 1, cannot see host processes
- `new_ipc_namespace`: Isolates IPC resources (message queues, semaphores, shared memory)
- `new_uts_namespace`: Isolates hostname and domain name

### User/Group Isolation

Run commands as a specific user or group:

```json
{
  "type": "exec",
  "spec": {
    "name": "run_as_nobody",
    "command": "whoami",
    "isolation": {
      "run_as_user": "nobody",
      "run_as_group": "nogroup"
    }
  }
}
```

**Note:** Requires root privileges to change to a different user.

### Resource Limits

Limit memory and CPU usage:

```json
{
  "type": "exec",
  "spec": {
    "name": "resource_limited",
    "command": "./memory-intensive-task.sh",
    "shell": true,
    "isolation": {
      "memory_limit_mb": 512,
      "cpu_quota": 100
    }
  }
}
```

- `memory_limit_mb`: Maximum memory in megabytes
- `cpu_quota`: CPU time limit (100 = 1 core, 200 = 2 cores)

### Complete Isolation Example

Maximum isolation for running untrusted code:

```json
{
  "type": "exec",
  "spec": {
    "name": "sandboxed_execution",
    "command": "./untrusted-script.sh",
    "shell": true,
    "timeout": 60,
    "isolation": {
      "disable_network": true,
      "new_pid_namespace": true,
      "new_ipc_namespace": true,
      "new_uts_namespace": true,
      "run_as_user": "nobody",
      "run_as_group": "nogroup",
      "memory_limit_mb": 256,
      "cpu_quota": 50
    }
  }
}
```

## Response Structure

The exec step stores a response object in the workflow environment:

```json
{
  "exit_code": 0,
  "stdout": "command output",
  "stderr": "error output",
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
      "exit_code": "{{my_command.exit_code}}",
      "output": "{{my_command.output}}",
      "stdout": "{{my_command.stdout}}"
    }
  }
}
```

## Configuration Options

### Core Options

| Field         | Type              | Description                   | Default     |
|---------------|-------------------|-------------------------------|-------------|
| `name`        | string            | Step name (required)          | -           |
| `command`     | string            | Command to execute (required) | -           |
| `args`        | []string          | Command arguments             | []          |
| `shell`       | bool              | Run through shell (`sh -c`)   | false       |
| `working_dir` | string            | Working directory             | current dir |
| `env`         | map[string]string | Environment variables         | {}          |
| `timeout`     | int               | Timeout in seconds            | 60          |

### Isolation Options (Linux only)

| Field                         | Type   | Description                   | Default       |
|-------------------------------|--------|-------------------------------|---------------|
| `isolation.disable_network`   | bool   | Disable network access        | false         |
| `isolation.new_pid_namespace` | bool   | New PID namespace             | false         |
| `isolation.new_ipc_namespace` | bool   | New IPC namespace             | false         |
| `isolation.new_uts_namespace` | bool   | New UTS namespace             | false         |
| `isolation.run_as_user`       | string | User to run as (name or UID)  | current user  |
| `isolation.run_as_group`      | string | Group to run as (name or GID) | current group |
| `isolation.memory_limit_mb`   | int64  | Memory limit in MB            | unlimited     |
| `isolation.cpu_quota`         | int64  | CPU quota (100 = 1 core)      | unlimited     |

## Platform Support

| Feature              | Linux | macOS | Windows |
|----------------------|-------|-------|---------|
| Basic execution      | ✅     | ✅     | ✅       |
| Shell mode           | ✅     | ✅     | ✅       |
| JSON parsing         | ✅     | ✅     | ✅       |
| Network isolation    | ✅     | ❌     | ❌       |
| Process namespaces   | ✅     | ❌     | ❌       |
| User/group isolation | ✅     | ❌     | ❌       |
| Resource limits      | ✅     | ❌     | ❌       |

## Security Considerations

1. **Untrusted Input**: Always use isolation when executing commands with user-provided input
2. **Network Access**: Disable network for build/test operations that don't need it
3. **Resource Limits**: Set memory/CPU limits to prevent resource exhaustion
4. **User Isolation**: Run as non-privileged user when possible
5. **Permissions**: Namespace isolation requires CAP_SYS_ADMIN or root privileges

## Error Handling

Commands that fail (non-zero exit code) will cause the workflow step to error:

```json
{
  "exit_code": 1,
  "stdout": "",
  "stderr": "error: file not found",
  "error": "command failed with exit code 1: exit status 1"
}
```

The workflow will stop at the failed step unless error handling is implemented.

## Best Practices

1. **Use Timeouts**: Always set reasonable timeouts for commands
2. **Isolate Untrusted Code**: Use full isolation for any untrusted scripts
3. **Limit Resources**: Set memory/CPU limits to prevent resource exhaustion
4. **Parse JSON**: Leverage automatic JSON parsing for structured output
5. **Check Exit Codes**: Use conditional steps to handle failures
6. **Log Everything**: Stdout and stderr are captured for debugging

## Examples

### Safe Package Installation

```json
{
  "type": "exec",
  "spec": {
    "name": "install_deps",
    "command": "npm install --production",
    "shell": true,
    "working_dir": "/app",
    "timeout": 300,
    "isolation": {
      "disable_network": false,
      "memory_limit_mb": 1024,
      "run_as_user": "node"
    }
  }
}
```

### Running Tests in Isolation

```json
{
  "type": "exec",
  "spec": {
    "name": "run_tests",
    "command": "npm test",
    "shell": true,
    "working_dir": "/app",
    "timeout": 600,
    "isolation": {
      "disable_network": true,
      "new_pid_namespace": true,
      "memory_limit_mb": 512
    }
  }
}
```

### Building Docker Images

```json
{
  "type": "exec",
  "spec": {
    "name": "build_image",
    "command": "docker",
    "args": ["build", "-t", "{{image_name}}:{{tag}}", "."],
    "working_dir": "/app",
    "timeout": 1800,
    "env": {
      "DOCKER_BUILDKIT": "1"
    }
  }
}
```
