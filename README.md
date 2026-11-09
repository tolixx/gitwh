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
- Copy the configuration file to `/etc/gitwh.yaml` (mode 640, it holds the secrets)
- Enable and start the systemd service

The service runs `git pull` as the user who ran the script (`$SUDO_USER` under sudo). That user must own the repo folders, git refuses to pull in a repo owned by someone else. To pick another user:

```bash
SERVICE_USER=deploy ./install.sh
```

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
    branch: "main"
    folders:
      - "/path/to/local/repo"
      - "/path/to/another/repo"
    after_pull: "bash tools/deploy.sh apply"
    after_pull_timeout: 300
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
- `timeout`: Git pull timeout in seconds, must be positive (default: `10`)
- `repos`: Map of repository configurations
  - `secret`: Optional webhook secret, the same value you enter in GitHub or GitLab (see Secrets below)
  - `branch`: Optional. Pull only on pushes to this branch, ignore the rest
  - `folders`: Array of local repository paths to pull, at least one
  - `after_pull`: Optional shell command run after a pull (see After pull below)
  - `after_pull_timeout`: Seconds before the command is killed (default: `300`)

Unknown keys are an error, so a typo like `folder:` fails at startup instead of being ignored.

### After pull

`after_pull` runs with `/bin/sh -c` in each folder of the repo, as the service user, after a `git pull` that moved `HEAD`. A relative path like `tools/deploy.sh` points into the pulled repo.

- It does not run when the pull failed or brought nothing new. A redelivery of the same push will not rerun a failed command: push a new commit or run it by hand.
- It runs in the folder's worker, so the next pull of that folder waits for it.
- Environment: `GITWH_REPO`, `GITWH_FOLDER`, `GITWH_OLD_HEAD`, `GITWH_NEW_HEAD`, plus the service's own.
- Output (the last 8 KB) and the exit status go to the log. On timeout the whole process group is killed, children included.
- Nothing from the webhook payload reaches the command, it is taken from the config only.
- A folder may get `after_pull` from one repo only.

### Secrets

- **GitHub** never sends the secret itself. It signs the body with it and sends the signature in `X-Hub-Signature-256`; gitwh checks that signature. Both content types (`application/json` and `application/x-www-form-urlencoded`) work.
- **GitLab** sends the secret token as is in `X-Gitlab-Token`; gitwh compares it.

Without a secret anyone who can reach `/wh` and knows the repo name can trigger a pull. Set one if the port is reachable from the internet.

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
2. **Payload Processing**: Tells GitHub from GitLab by the `X-GitHub-Event` / `X-Gitlab-Event` header and parses the payload
3. **Repository Validation**: Checks that the repository is configured and verifies the signature or token if a secret is set
4. **Filtering**: Pings, non-push events, tag pushes and pushes to other branches (when `branch` is set) get `202` and no pull
5. **Git Pull**: Executes `git pull` on configured local repository paths, git output goes to the log on failure
6. **After Pull**: Runs the repo's `after_pull` command in the folder if the pull brought new commits
7. **Concurrency Control**: At most one pull per folder at a time. Pushes that arrive during a pull collapse into one follow-up pull

## API Endpoints

- `GET /`: Returns 404 Not Found
- `POST /wh`: Webhook endpoint for GitHub/GitLab push events. `200` pull queued, `202` delivery accepted but ignored, `400` unknown repo, bad payload or bad secret

## Architecture

The application is structured into several packages:

- `main.go`: Entry point and HTTP server setup
- `config/`: Configuration loading and parsing
- `handlers/`: HTTP request handling and webhook processing
- `puller/`: Git pull interface and implementation
- `puller/git/`: Git-specific pull implementation with concurrency control

## Security

- GitHub HMAC signature and GitLab token validation, constant-time comparison
- Request body limit and HTTP server timeouts
- Configurable git timeouts, git never waits for a password prompt
- Mutex-based concurrency control to prevent race conditions
- Minimal attack surface with only one webhook endpoint

## Dependencies

- `github.com/go-chi/chi/v5`: HTTP router and middleware
- `gopkg.in/yaml.v3`: YAML configuration parsing

## License

This project is provided as-is without any specific license terms.
