# gitdeck

A terminal UI for monitoring CI/CD pipelines. Run it from any git repository and get an instant overview of pipeline runs and job statuses — no browser required.

Supports **GitHub Actions** and **GitLab CI/CD** (cloud and self-hosted).

```
 gitdeck  waabox/gitdeck  q:quit  r:refresh
────────────────────────────────────────────────────────────
 PIPELINES
  ● main  05ae12b  "feat: read OAuth client IDs from config"  waabox   2m
  ✓ main  efb251c  "fix: move Token saved message"            waabox  45s
  ✓ main  0b73eb0  "fix: use stderr for auth prompts"         waabox   1m

 JOBS
  ✓ build    build    12s
  ✓ test     test     38s
  ● lint     lint      3s

────────────────────────────────────────────────────────────
 #1042  main  05ae12b  "feat: read OAuth client IDs"  by waabox
 j/k: navigate   tab: switch panel   enter: select   r: refresh   q: quit
```

## Features

- Live pipeline list with status icons and durations
- Job detail panel for the selected pipeline
- Auto-refresh every 30 seconds
- OAuth Device Flow authentication for GitHub and GitLab (no manual token copy-paste)
- Config via `~/.config/gitdeck/config.toml` with environment variable overrides
- Auto-detects repository from the current working directory

## Installation

### From source

```bash
git clone https://github.com/waabox/gitdeck.git
cd gitdeck
go build -o gitdeck ./cmd/gitdeck
mv gitdeck /usr/local/bin/
```

Requires Go 1.24 or later.

## Configuration

Create `~/.config/gitdeck/config.toml`:

```toml
[github]
client_id = "YOUR_GITHUB_OAUTH_APP_CLIENT_ID"
# token is written here automatically after first login

[gitlab]
client_id = "YOUR_GITLAB_OAUTH_APP_CLIENT_ID"
# url is only needed for self-hosted GitLab instances
# url = "https://gitlab.example.com"
# token is written here automatically after first login
```

### Environment variable overrides

| Variable       | Overrides        |
|----------------|------------------|
| `GITHUB_TOKEN` | `github.token`   |
| `GITLAB_TOKEN` | `gitlab.token`   |
| `GITLAB_URL`   | `gitlab.url`     |

### Setting up OAuth Apps

**GitHub**: Create an OAuth App at *Settings → Developer settings → OAuth Apps*. Set the callback URL to `http://localhost` (not used for device flow). Copy the Client ID into the config.

**GitLab**: Create an OAuth App at *User Settings → Applications*. Enable the `read_api` scope and tick *Allow Device Authorization Grant*. Copy the Application ID into the config.

## Authentication

On first run, if no token is configured, gitdeck starts the OAuth Device Flow automatically:

```
No GitHub token found. Starting OAuth authentication...
Visit:      https://github.com/login/device
Enter code: ABCD-1234
Waiting for authorization...
Authenticated. Token saved to /Users/you/.config/gitdeck/config.toml
```

The token is saved to the config file so subsequent runs are silent.

## Keyboard shortcuts

| Key          | Action                                  |
|--------------|-----------------------------------------|
| `j` / `↓`   | Move down in the pipeline list          |
| `k` / `↑`   | Move up in the pipeline list            |
| `Enter`      | Select pipeline, focus job detail panel |
| `Tab`        | Switch focus between panels             |
| `Esc`        | Return focus to the pipeline list       |
| `r`          | Refresh pipelines now                   |
| `q` / `Ctrl+C` | Quit                                  |

## Usage

```bash
# Run from inside any git repository
cd /path/to/your/project
gitdeck
```

gitdeck reads the `origin` remote from `.git/config` and selects the correct CI provider automatically.

## License

MIT — see [LICENSE](LICENSE).
