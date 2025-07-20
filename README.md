# GitWH - Git WebHook Server

A lightweight webhook server written in Go that automatically pulls Git repositories when receiving push notifications from GitHub or GitLab.
This software provides only one function - updates local copy ( using git pull ) of git repository.

## Features

- Support for both GitHub and GitLab webhooks
- Automatic `git pull` on configured repositories
- Configurable buffer size and timeout settings
- Secret validation for enhanced security
- Concurrent request handling with per-directory mutex locks
- Systemd service integration
- Support for both JSON and YAML configuration formats

## Installation

### Prerequisites

- Go 1.20 or higher
- Git installed on the system
- Systemd (for service installation)

### Building

```bash
go install
```

This command compiles and installs binary into default go bin path. You should add this path to your PATH variable before run install script.
Or you could build binary by command

```bash
go build -o <path_to_binary>/gitwh
```
It's necessary to add <path_to_binary> to your PATH environment variable.

### Service Installation

Use the provided installation script to set up the service:

```bash
./install.sh
```

This script will:
- Copy the service file to `/etc/systemd/system/`
- Copy the configuration file to `/etc/gitwh.yaml`
- Enable and start the systemd service

## Configuration

The server supports both YAML and JSON configuration formats. By default, it looks for the configuration file at `/etc/gitwh.yaml`.

### YAML Configuration Example

```yaml
listen: ":8080"
buffer_size: 3
timeout: 10
repos:
  my-repo:
    secret: "optional-webhook-secret"
    folders:
      - "/path/to/local/repo"
      - "/path/to/another/repo"
```

my-repo - name of your repository. In case of https://github.com/tolixx/gitwh it is gitwh.
folders - path to your local copy of this repo.


### JSON Configuration Example

```json
{
  "listen": ":8080",
  "buffer_size": 3,
  "timeout": 10,
  "repos": {
    "my-repo": {
      "secret": "optional-webhook-secret",
      "folders": ["/path/to/local/repo"]
    }
  }
}
```

### Configuration Parameters

- `listen`: Server listening address and port (default: `:8080`)
- `buffer_size`: Event buffer size for concurrent requests (default: `3`)
- `timeout`: Git pull timeout in seconds (default: `10`)
- `repos`: Map of repository configurations
  - `secret`: Optional webhook secret for validation
  - `folders`: Array of local repository paths to pull

## Usage

### Command Line

```bash
# Use default config file (/etc/gitwh.yaml)
./gitwh

# Use custom config file
./gitwh -config /path/to/config.yaml
```

### Webhook URL

Set up webhooks in your GitHub/GitLab repository to point to:

```
http://your-server:8080/wh
```

### Service Management

```bash
# Check service status
sudo systemctl status gitwh.service

# Start service
sudo systemctl start gitwh.service

# Stop service
sudo systemctl stop gitwh.service

# Restart service
sudo systemctl restart gitwh.service

# View logs
sudo journalctl -u gitwh.service -f
```

## How It Works

1. **Webhook Reception**: The server listens for HTTP POST requests on the `/wh` endpoint
2. **Payload Processing**: Automatically detects and parses GitHub or GitLab webhook payloads
3. **Repository Validation**: Checks if the repository is configured and validates secrets if provided
4. **Git Pull**: Executes `git pull` on configured local repository paths
5. **Concurrency Control**: Uses per-directory mutex locks to prevent concurrent pulls on the same repository

## API Endpoints

- `GET /`: Returns 404 Not Found
- `POST /wh`: Webhook endpoint for GitHub/GitLab push events

## Architecture

The application is structured into several packages:

- `main.go`: Entry point and HTTP server setup
- `config/`: Configuration loading and parsing
- `handlers/`: HTTP request handling and webhook processing
- `puller/`: Git pull interface and implementation
- `puller/git/`: Git-specific pull implementation with concurrency control

## Security

- Secret validation for webhook requests
- Configurable timeouts to prevent hanging operations
- Mutex-based concurrency control to prevent race conditions
- Minimal attack surface with only one webhook endpoint

## Dependencies

- `github.com/go-chi/chi/v5`: HTTP router and middleware
- `gopkg.in/yaml.v3`: YAML configuration parsing

## License

This project is provided as-is without any specific license terms.
