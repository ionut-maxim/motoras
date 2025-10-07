# Git Subscriber

The Git subscriber monitors a Git repository for new commits and tags, emitting events when changes are detected.

## Features

- Monitor commits on a specific branch
- Optionally monitor for new tags
- Supports both local and remote repositories
- Configurable polling interval
- Deduplication - ensures each commit/tag only triggers once

## Configuration

```json
{
  "repository": "/path/to/repo",
  "branch": "main",
  "watch_tags": true,
  "poll_interval": "30s"
}
```

### Parameters

| Parameter       | Type            | Required | Default         | Description                                              |
|-----------------|-----------------|----------|-----------------|----------------------------------------------------------|
| `repository`    | string          | Yes      | -               | Path to local repository or remote URL to clone          |
| `branch`        | string          | No       | `"main"`        | Branch to monitor for commits                            |
| `watch_tags`    | boolean         | No       | `false`         | Whether to watch for new tags                            |
| `poll_interval` | string/duration | No       | `"30s"`         | How often to check for changes (e.g., "10s", "1m", "5m") |
| `auth_method`   | string          | No       | -               | Authentication method: `"ssh"` or `"token"`              |
| `ssh_key_path`  | string          | No       | `~/.ssh/id_rsa` | Path to SSH private key (for `auth_method: "ssh"`)       |
| `ssh_password`  | string          | No       | -               | Password for encrypted SSH key                           |
| `username`      | string          | No       | -               | Username for token authentication                        |
| `token`         | string          | No       | -               | Personal access token or password                        |

## Event Payloads

### Commit Events

When a new commit is detected, the following payload is emitted:

```json
{
  "type": "commit",
  "sha": "9a61fbadadb9925728b413ac107e597cab671fcb",
  "short_sha": "9a61fba",
  "branch": "main",
  "author": "John Doe",
  "author_email": "john@example.com",
  "committer": "John Doe",
  "committer_email": "john@example.com",
  "message": "Add new feature",
  "timestamp": 1759845404
}
```

### Tag Events

When a new tag is detected (if `watch_tags` is enabled):

```json
{
  "type": "tag",
  "tag": "v1.0.0",
  "tag_sha": "f464a73adddd2f4a7aaf56ced439a00055b76533",
  "commit_sha": "3c7ce2657be4e26498d98b14dafcd670624749ba",
  "message": "Release v1.0.0",
  "author": "John Doe",
  "author_email": "john@example.com",
  "timestamp": 1759845417
}
```

## Example Triggers

### Local Repository

Monitor a local repository for commits and tags:

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "monitor-myproject-repo",
  "type": "git",
  "workflow_id": "660e8400-e29b-41d4-a716-446655440000",
  "data": {
    "repository": "/home/user/projects/myproject",
    "branch": "main",
    "watch_tags": true,
    "poll_interval": "60s"
  }
}
```

### Private GitHub Repository with Token

Monitor a private GitHub repository using a personal access token:

```json
{
  "id": "660e8400-e29b-41d4-a716-446655440001",
  "name": "monitor-private-github-repo",
  "type": "git",
  "workflow_id": "770e8400-e29b-41d4-a716-446655440000",
  "data": {
    "repository": "https://github.com/myorg/private-project.git",
    "branch": "main",
    "watch_tags": true,
    "poll_interval": "5m",
    "auth_method": "token",
    "username": "myusername",
    "token": "ghp_xxxxxxxxxxxxxxxxxxxx"
  }
}
```

### Private Repository with SSH

Monitor a private repository using SSH authentication:

```json
{
  "id": "770e8400-e29b-41d4-a716-446655440002",
  "name": "monitor-private-gitlab-repo",
  "type": "git",
  "workflow_id": "880e8400-e29b-41d4-a716-446655440000",
  "data": {
    "repository": "git@gitlab.com:myorg/private-api.git",
    "branch": "production",
    "watch_tags": true,
    "poll_interval": "2m",
    "auth_method": "ssh",
    "ssh_key_path": "/home/app/.ssh/deploy_key"
  }
}
```

## Usage with Remote Repositories

### Public Repositories

For public remote repositories, the subscriber will automatically clone the repository to a temporary directory on first
run:

```json
{
  "repository": "https://github.com/username/repo.git",
  "branch": "develop",
  "poll_interval": "5m"
}
```

### Private Repositories

For private repositories, you need to configure authentication:

#### SSH Authentication

Using SSH with a private key:

```json
{
  "repository": "git@github.com:username/private-repo.git",
  "branch": "main",
  "auth_method": "ssh",
  "ssh_key_path": "/home/user/.ssh/id_ed25519"
}
```

If your SSH key is encrypted with a password:

```json
{
  "repository": "git@github.com:username/private-repo.git",
  "branch": "main",
  "auth_method": "ssh",
  "ssh_key_path": "/home/user/.ssh/id_rsa",
  "ssh_password": "my-key-password"
}
```

#### Token/Password Authentication

Using a personal access token (recommended for GitHub, GitLab, etc.):

```json
{
  "repository": "https://github.com/username/private-repo.git",
  "branch": "main",
  "auth_method": "token",
  "username": "your-username",
  "token": "ghp_YourPersonalAccessToken"
}
```

**Note**: For GitHub, you can create a personal access token at: Settings → Developer settings → Personal access tokens

## Best Practices

1. **Polling Interval**: Choose an appropriate polling interval based on your needs:
    - Active development: 30s - 2m
    - Production monitoring: 5m - 15m
    - Archive/slow-moving repos: 30m - 1h

2. **Branch Selection**: Monitor specific branches to avoid noise from feature branches

3. **Tag Watching**: Enable `watch_tags` only if you need to react to releases/deployments

4. **Remote vs Local**:
    - Use local paths for repos on the same machine
    - Use URLs for monitoring remote repositories (will be cloned locally)

## Limitations

- For remote repositories, changes are detected via polling (no webhook support)
- The repository is cloned/opened once at startup
- Credentials are stored in trigger configuration (consider using environment variables or secrets management for
  sensitive data)

## Security Considerations

1. **Protect your credentials**: Tokens and passwords are stored in the trigger configuration. Use secrets management or
   environment variables when possible.

2. **SSH keys**: If using SSH authentication, ensure the private key file has appropriate permissions (typically `600`).

3. **Token scope**: When creating personal access tokens, grant only the minimum required permissions (typically just
   `repo` read access).

4. **Rotation**: Regularly rotate your tokens and SSH keys as part of your security practices.
