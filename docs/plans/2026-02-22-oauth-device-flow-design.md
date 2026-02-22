# OAuth Device Authorization Flow — Design

**Date:** 2026-02-22
**Status:** Approved

## Summary

Implement the OAuth 2.0 Device Authorization Flow (RFC 8628) for both GitHub and GitLab
so that users no longer need to manually create and configure personal access tokens.
When `gitdeck` starts and finds no token for the detected provider, it automatically
initiates the device flow inline, then saves the obtained token to the config file.

---

## Goals

- Zero-config authentication: no manual token creation required
- Works for both GitHub Actions and GitLab CI (including self-hosted GitLab instances)
- Token is persisted after first auth so subsequent runs are seamless
- Existing token-based config (env vars, manual TOML entries) continues to work unchanged

---

## Flow

```
1. Detect git remote → identify provider (github.com / gitlab.com / self-hosted)
2. Load ~/.config/gitdeck/config.toml
3. If the relevant token field is empty:
     a. Start Device Flow for that provider
     b. Print: "Visit: <url>"  +  "Enter code: <user_code>"
     c. Poll token endpoint until access_token received or error
     d. Write token back to config file via config.Save()
4. Build provider adapter with the token
5. Launch TUI
```

---

## New Package: `internal/auth`

### `device.go` — shared types

```go
// DeviceCodeResponse holds the initial response from a device code request.
type DeviceCodeResponse struct {
    DeviceCode      string
    UserCode        string
    VerificationURI string
    ExpiresIn       int  // seconds
    Interval        int  // polling interval in seconds
}

// DeviceFlow is the contract for provider-specific device authorization flows.
type DeviceFlow interface {
    RequestCode() (DeviceCodeResponse, error)
    PollToken(deviceCode string, interval int) (string, error)
}
```

Poll errors to handle:
- `authorization_pending` — keep waiting
- `slow_down` — increase interval by 5s
- `expired_token` — return error, prompt user to restart
- `access_denied` — return error

### `github.go` — `GitHubDeviceFlow`

| Step | Endpoint | Method | Notes |
|------|----------|--------|-------|
| Request code | `https://github.com/login/device/code` | POST | `client_id`, `scope=repo,workflow` |
| Poll token | `https://github.com/login/oauth/access_token` | POST | `client_id`, `device_code`, `grant_type=urn:ietf:params:oauth:grant-type:device_code` |

- Embedded constant: `githubClientID` (placeholder — set after registering OAuth App)
- No client secret required for device flow on GitHub

### `gitlab.go` — `GitLabDeviceFlow`

| Step | Endpoint | Method | Notes |
|------|----------|--------|-------|
| Request code | `<baseURL>/oauth/authorize_device` | POST | `client_id`, `scope=read_api` |
| Poll token | `<baseURL>/oauth/token` | POST | `client_id`, `device_code`, `grant_type=urn:ietf:params:oauth:grant-type:device_code` |

- `baseURL` defaults to `https://gitlab.com`, uses `cfg.GitLab.URL` for self-hosted
- Embedded constant: `gitlabClientID` (placeholder — set after registering OAuth App)

---

## Config Changes

Add `Save(path string) error` to `internal/config/config.go`:
- Marshals the `Config` struct to TOML
- Creates parent directory (`~/.config/gitdeck/`) if it does not exist
- Overwrites the file atomically (write to temp, rename)

---

## main.go Changes

After loading config and detecting the provider, insert an auth check before building
the adapter:

```go
if providerHost == "github.com" && cfg.GitHub.Token == "" {
    token, err := runGitHubDeviceFlow()
    // print instructions, poll, handle errors
    cfg.GitHub.Token = token
    config.Save(configPath, cfg)
}
// same pattern for gitlab
```

---

## OAuth App Registration (Prerequisites)

Before compiling, the user must:

1. **GitHub**: Register an OAuth App at `https://github.com/settings/developers`
   - Enable "Device Flow" in the app settings
   - Set the embedded `githubClientID` constant in `internal/auth/github.go`

2. **GitLab**: Register an Application at `https://gitlab.com/-/user_settings/applications`
   - Grant type: "Device Authorization Grant"
   - Scope: `read_api`
   - Set the embedded `gitlabClientID` constant in `internal/auth/gitlab.go`

---

## Error Handling

| Scenario | Behavior |
|----------|----------|
| User denies access | Print error, exit with non-zero code |
| Code expires | Print error, prompt to run again |
| Network error during poll | Retry up to 3 times, then exit |
| Token write fails | Print warning, continue with in-memory token for this session |

---

## Testing

- Unit tests for `GitHubDeviceFlow` and `GitLabDeviceFlow` using an `httptest.Server`
- Test all poll states: `authorization_pending`, `slow_down`, `expired_token`, `access_denied`, success
- Unit test for `config.Save` roundtrip (save then load, verify tokens match)
