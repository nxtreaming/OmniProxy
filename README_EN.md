# 🚀 OmniProxy

> A local AI API token scheduler, quota monitor, and transparent proxy gateway. Manage multiple accounts, multiple providers, and multiple AI coding clients from one desktop app.

English · [中文](README.md)

OmniProxy is built for local AI development workflows. Codex, Claude Code, and OpenAI / Anthropic-compatible clients can connect to a local proxy, while OmniProxy chooses an available account, injects credentials, records usage, refreshes quotas, retries transient failures, and switches accounts when needed.

OmniProxy is useful when you run into problems like:

- 🪫 One account runs out of quota and the client fails immediately.
- 🔁 You keep switching accounts by editing local config files.
- 🧩 OpenAI, Anthropic, DeepSeek, Kimi, and Xiaomi MiMo all use different endpoints and auth styles.
- 🕵️ You cannot tell which account handled a request, how many tokens were used, or where a failure happened.
- 🛠️ You want Codex / Claude Code config to be written locally and restored safely.

## ✨ Highlights

- 🖥️ **Desktop control loop**: Wails + Go + Vue 3 for account, proxy, quota, logs, and settings management.
- 🔐 **Local transparent proxy**: clients talk to `127.0.0.1`; real upstream credentials are injected by OmniProxy.
- 🧠 **Multi-account scheduling**: queue mode and balanced mode, with in-flight account avoidance.
- 🎯 **Account selection scheduling**: providers rotate all usable accounts by default; once one or more accounts are selected, scheduling is limited to that selected set.
- 🧯 **Automatic failover**: retry with another usable account on `429`, `502`, `503`, and `504`.
- 📊 **Quota and usage visibility**: remaining quota, reset time, request counts, input / output / total tokens.
- 📈 **History analytics**: summarize request history by date, provider, model, and failure reason, including a model token pie chart.
- ⚡ **Active account quota refresh**: Codex and verifiable active accounts refresh quota state automatically every 30 seconds.
- 🧭 **Claude Code routing**: route Claude Code locally to DeepSeek, Kimi, or Xiaomi MiMo.
- 🧵 **Codex WebSocket proxy**: optional Codex WebSocket proxying with usage logging.
- 💬 **OpenRouter chat and models**: refresh OpenRouter model lists and run quick desktop chat checks.
- 🧱 **Local persistence**: config, accounts, and usage stats are stored locally; on Windows, account credentials are encrypted at rest with DPAPI.
- 📤 **Credential export**: export a full account-pool backup or export Codex auth.json values as separate files.
- 🎨 **Polished desktop UX**: page transitions, highlighted active account, navigation icons, and a custom app icon.

## 🧠 How It Works

```text
Codex / Claude Code / API Client
              |
              v
     http://127.0.0.1:3000
              |
              v
        OmniProxy Gateway
   Token Pool + Scheduler + Logs
              |
              v
OpenAI / Anthropic / DeepSeek / Kimi / Xiaomi MiMo
Zhipu GLM / MiniMax / Gemini / OpenRouter / Custom Gateway
```

OmniProxy does not require clients to know which real account is being used. Point your client to the local proxy, and OmniProxy handles account selection, auth header injection, retries, quota updates, and logs.

## 📦 Project Structure

```text
.
├── OmniProxyBackend/              # Wails desktop app and Go backend
│   ├── internal/config/           # Local config, data directory, defaults
│   ├── internal/logs/             # In-memory log recorder
│   ├── internal/proxy/            # Proxy, routes, auth, usage parsing, WebSocket
│   ├── internal/storage/          # JSON local storage
│   ├── internal/token/            # Token pool, scheduling, status, stats
│   └── frontend/                  # Vue 3 + Vite + Element Plus frontend
├── scripts/dev.ps1                # Wails desktop development script
├── scripts/build-dev.ps1          # Build a Dev exe that can coexist with production
├── docs/releases/                 # Release notes
├── README.md                      # Chinese README
└── README_EN.md                   # English README
```

Default ports and data location:

| Item | Production | Dev |
| --- | --- | --- |
| 🧩 Control API | `http://127.0.0.1:3890/api` | `http://127.0.0.1:3891/api` |
| 🚪 Proxy server | `http://127.0.0.1:3000` | `http://127.0.0.1:3001` |
| 💾 Local data | `%USERPROFILE%\.omniproxy` | `%USERPROFILE%\.omniproxy-dev` |
| 🧭 Bootstrap file | `%USERPROFILE%\.omniproxy-bootstrap.json` | `%USERPROFILE%\.omniproxy-dev-bootstrap.json` |

## ⚡ Quick Start

### 1. Requirements

- Go
- Node.js
- Wails v2 CLI

### 2. Run in Development

```powershell
cd .\OmniProxyBackend
C:\Users\mimanchi\go\bin\wails.exe dev
```

Or use the helper script:

```powershell
.\scripts\dev.ps1
```

### 3. Build the Production Desktop App

```powershell
cd .\OmniProxyBackend
C:\Users\mimanchi\go\bin\wails.exe build
```

Build output:

```powershell
.\OmniProxyBackend\build\bin\OmniProxy.exe
```

### 4. Build a Coexisting Dev exe

The Dev build uses the `omniproxy_dev` build tag. It shows as `OmniProxy Dev` and uses a separate single-instance ID, data directory, and default ports. This is the preferred build when you want to test new functionality while a production install is still present.

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build-dev.ps1 -Clean
```

Build output:

```powershell
.\OmniProxyBackend\build\bin\OmniProxy-Dev.exe
```

You can also set the displayed version:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build-dev.ps1 -Version dev-issue-4
```

## 🧭 Usage

1. 🚀 Start OmniProxy.
2. 🔑 Add OpenAI, Anthropic, DeepSeek, Kimi, Xiaomi MiMo, Zhipu, MiniMax, Gemini, OpenRouter, or custom gateway accounts in **Account Management**.
3. ⚙️ Confirm proxy port and provider Base URLs in **Global Settings**.
4. 🟢 Start the local proxy.
5. 🧩 Point Codex, Claude Code, or your API client to the local proxy.
6. 🎯 To limit routing, click **Select** on quota cards. With no selected accounts, the provider rotates all usable accounts; with one or more selected accounts, it rotates only within that selected set. Enable / disable accounts from **Account Management**.
7. 📊 Use the dashboard, quota, request history, and billing pages to inspect the active account, reset time, token usage, and live logs.

Common local endpoints:

```text
OpenAI compatible: http://127.0.0.1:3000
Codex backend:     http://127.0.0.1:3000/backend-api/codex
Claude router:     http://127.0.0.1:3000/anthropic-router
```

The Dev build shifts the default ports:

```text
OpenAI compatible: http://127.0.0.1:3001
Codex backend:     http://127.0.0.1:3001/backend-api/codex
Claude router:     http://127.0.0.1:3001/anthropic-router
```

The desktop app also includes one-click setup actions for local Codex and Claude Code configuration, with restore support for previous config files.

## 🔌 Supported Credentials

| Provider | Credential | Notes |
| --- | --- | --- |
| OpenAI | API Key | Uses `Authorization: Bearer` |
| OpenAI / Codex | `auth.json` | Parses email, access token, account id, and refreshes Codex quota |
| Anthropic | API Key | Uses `x-api-key` |
| Anthropic / Claude | OAuth JSON | Supports Claude OAuth JSON with `access_token` / `refresh_token` |
| DeepSeek | API Key | Supports OpenAI-compatible and Anthropic routing |
| Kimi | API Key | Supports Kimi Code routing |
| Xiaomi MiMo | API Key | Pay-as-you-go key, usually starts with `sk-` |
| Xiaomi MiMo | Token Plan | Token Plan key, usually starts with `tp-` |
| Zhipu GLM | API Key | Supports OpenAI-compatible and Anthropic routing |
| Zhipu GLM | Coding Plan | Refreshes Coding Plan usage from the subscription quota endpoint |
| MiniMax | API Key | Supports OpenAI-compatible and Anthropic routing |
| Gemini | API Key | Supports Gemini API routing |
| OpenRouter | API Key | Supports model refresh, balance checks, and desktop chat |
| Custom Gateway | API Key | Supports OpenAI / Anthropic-compatible gateways |

## 🧰 Control API

The desktop frontend prefers Wails bindings. The local HTTP control API is still available for debugging, scripts, and compatibility with older development flows:

- `GET /api/control-token`
- `GET /api/tokens`
- `POST /api/tokens`
- `PUT /api/tokens/{id}`
- `DELETE /api/tokens/{id}`
- `PUT /api/tokens/{id}/disabled`
- `PUT /api/tokens/{id}/selected`
- `PUT /api/tokens/{id}/exclusive`
- `DELETE /api/tokens/{id}/exclusive`
- `POST /api/tokens/{id}/validate`
- `GET /api/config`
- `PUT /api/config`
- `GET /api/logs`
- `GET /api/history`
- `POST /api/history/clear`
- `GET /api/billing/usage`
- `GET /api/billing/dates`
- `POST /api/billing/clear`
- `GET /api/proxy/status`
- `GET /api/proxy/active-requests`
- `POST /api/proxy/start`
- `POST /api/proxy/stop`
- `GET /api/app/info`
- `POST /api/update/check`
- `POST /api/update/download`
- `GET /api/update/download/status`
- `POST /api/update/install`
- `GET /api/data-directory`
- `PUT /api/data-directory`
- `POST /api/codex/configure`
- `POST /api/codex/restore`
- `POST /api/mimo/claude/configure`
- `POST /api/mimo/claude/restore`
- `POST /api/deepseek/claude/configure`
- `POST /api/deepseek/claude/restore`
- `POST /api/kimi/claude/configure`
- `POST /api/kimi/claude/restore`
- `POST /api/zhipu/claude/configure`
- `POST /api/zhipu/claude/restore`
- `POST /api/gemini/configure`
- `POST /api/gemini/restore`
- `GET /api/openrouter/models`
- `POST /api/openrouter/chat`
- `POST /api/opencode/configure`
- `POST /api/opencode/restore`

`/selected` adds or removes an account from its provider's scheduling selection set. If a provider has no selected accounts, it rotates all usable accounts by default. `/exclusive` remains for compatibility and means clearing the provider selection set before selecting only the current account.

When using the HTTP control API, every endpoint except `GET /api/control-token` requires the `X-OmniProxy-Control-Token` header. `Authorization: Bearer <token>` is also accepted.

## ✅ Verification

Backend tests:

```powershell
cd .\OmniProxyBackend
go test ./...
```

Frontend tests:

```powershell
cd .\OmniProxyBackend\frontend
npm test
```

Frontend build:

```powershell
cd .\OmniProxyBackend\frontend
npm run build
```

Desktop package:

```powershell
cd .\OmniProxyBackend
C:\Users\mimanchi\go\bin\wails.exe build
```

Coexisting Dev exe:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build-dev.ps1 -Clean
```

## 🛡️ Security Notes

OmniProxy is designed for personal local development and binds to `127.0.0.1` by default. Credentials are stored in the local data directory you choose and are not uploaded by OmniProxy. On Windows, real credential values in `tokens.json` are encrypted with the current user's DPAPI profile; use the desktop export feature before moving credentials to another machine or user.

Recommendations:

- 🚫 Do not commit `auth.json`, `.env`, `tokens.json`, or real credentials.
- 🔒 Do not expose the control API to public networks or LAN environments.
- 🧪 Check that the local proxy port is not already used by another process.
- 📤 Exported account-pool backups and Codex auth.json files contain real credentials. Store them only in trusted directories.
- 🧹 Before sharing logs or screenshots, check for account names, paths, or sensitive config.

## 🗺️ Roadmap

- 📈 Add quota trend charts, request history charts, and deeper usage analytics.
- 🔐 Tighten the local control API security boundary.
- 🧩 Add more providers and protocol adapters.
- 🧪 Expand end-to-end coverage for SSE, WebSocket, concurrent scheduling, and recovery.
- 🧱 Continue splitting frontend state and page components.
- 📦 Improve desktop packaging and release workflow.

## 🤝 Contributing

Issues and pull requests are welcome.

When opening an Issue, please include:

- 🖥️ Operating system and run mode.
- 🧰 Client tool, such as Codex, Claude Code, or a custom API client.
- 🔌 Provider, route path, and relevant error logs.
- 🎯 Expected behavior and actual behavior.

Good first contribution areas:

- ✨ New provider or protocol adapters.
- 🧠 Proxy reliability, scheduling, and retry strategy improvements.
- 🎨 Frontend interaction, visualization, and configuration UX.
- ✅ Tests, documentation examples, and reproduction cases.

Before submitting a PR, please run:

```powershell
cd .\OmniProxyBackend
go test ./...
```

```powershell
cd .\OmniProxyBackend\frontend
npm test
```

```powershell
cd .\OmniProxyBackend\frontend
npm run build
```

## ⭐ Support

If OmniProxy helps your local AI development workflow, please consider giving it a Star. ⭐

Stars help more people with the same workflow pain discover the project, and real-world Issues / PRs help guide the roadmap better than any guesswork.
