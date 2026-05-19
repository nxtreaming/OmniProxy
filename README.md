# OmniProxy

<div align="center">

**本地优先的 AI API 网关、账号调度器与额度观测控制台**

把 Codex、Claude Code、OpenCode、Pi Coding Agent、Gemini CLI 以及 OpenAI / Anthropic 兼容客户端统一接入本机代理，由 OmniProxy 在本地完成账号选择、鉴权注入、失败重试、额度刷新、用量统计和客户端配置写入。

[English](README_EN.md) · [发布记录](docs/releases) · [Releases](https://github.com/mibgb65-cloud/OmniProxy/releases)

![Release](https://img.shields.io/github/v/release/mibgb65-cloud/OmniProxy?include_prereleases&label=release)
![Platform](https://img.shields.io/badge/platform-Windows-0078D4)
![Go](https://img.shields.io/badge/Go-1.26-00ADD8)
![Vue](https://img.shields.io/badge/Vue-3-42B883)
![Wails](https://img.shields.io/badge/Wails-v2-DF0000)

</div>

## 为什么需要 OmniProxy

本地 AI 开发工具越来越多，账号、Base URL、模型和额度状态却散落在不同配置文件里。OmniProxy 把这些分散状态收拢到一个本地桌面控制台中：

- 多个账号不再手动切换，由调度器按状态、选择范围和并发占用自动挑选。
- 客户端只连接 `127.0.0.1`，真实上游 Token 留在本机，由代理按厂商注入。
- Codex、Claude Code、OpenCode、Pi Coding Agent 等工具可以一键写入本地代理配置，并保留备份用于恢复。
- 请求历史、模型 Token、失败原因、额度重置时间和本地账单统计统一可见。

OmniProxy 不是云端中转服务。它面向个人本地开发场景，默认只监听 loopback 地址，凭据保存在本机数据目录中。

## 核心能力

| 能力 | 说明 |
| --- | --- |
| 本地透明代理 | 暴露 OpenAI、Anthropic、Codex、Pi、TokenRouter 等本地入口，自动注入上游鉴权。 |
| 多账号调度 | 支持队列模式、优先平衡使用、账号选择范围、低额度跳过和并发占用避让。 |
| 失败自动切换 | 上游返回 `429`、`502`、`503`、`504` 等可重试错误时，自动换账号重试。 |
| 额度观测 | 展示 API 余额、订阅额度、重置时间、Codex Free 周额度、Coding Plan 用量和 OpenRouter 余额。 |
| 用量统计 | 记录请求历史、客户端来源、模型、输入 / 输出 / 总 Token、失败原因和每日账单快照。 |
| 客户端配置 | 一键配置 Codex、Claude Code、Gemini CLI、OpenCode、Pi Coding Agent，并支持恢复原配置。 |
| Claude 模型槽位 | 可从 DeepSeek、MiMo、Kimi、GLM 模型中选择最多 4 个模型写入 Claude Code。 |
| 本地安全存储 | Windows 上使用当前用户 DPAPI 加密账号凭据，导出备份时保持显式可控。 |

## 架构

```mermaid
flowchart LR
  subgraph Clients["本地客户端"]
    Codex["Codex"]
    Claude["Claude Code"]
    OpenCode["OpenCode"]
    Pi["Pi Coding Agent"]
    API["OpenAI / Anthropic Client"]
  end

  subgraph OmniProxy["OmniProxy Desktop"]
    Console["Vue 控制台"]
    Proxy["Local Gateway :3000"]
    Scheduler["Token Pool + Scheduler"]
    Logs["History / Billing / Quota"]
  end

  subgraph Providers["上游服务"]
    OpenAI["OpenAI / Codex"]
    Anthropic["Anthropic"]
    DeepSeek["DeepSeek"]
    Kimi["Kimi"]
    Mimo["Xiaomi MiMo"]
    More["Zhipu / MiniMax / Gemini / OpenRouter / TokenRouter / Custom"]
  end

  Clients --> Proxy
  Console --> Scheduler
  Proxy --> Scheduler
  Scheduler --> Logs
  Scheduler --> OpenAI
  Scheduler --> Anthropic
  Scheduler --> DeepSeek
  Scheduler --> Kimi
  Scheduler --> Mimo
  Scheduler --> More
```

## 最新变化

- **Codex Chat Completions 兼容入口**：新增 `/codex/v1/chat/completions`，可用 OpenAI `auth.json` 账号接入 OpenAI Chat Completions 客户端，内部自动转换到 Codex Responses 后端。
- **Codex 流式响应转换**：Codex Responses 的 SSE 事件会转换为 `chat.completion.chunk`，非流式请求会汇总为 `chat.completion` 响应。
- **Codex 模型与参数适配**：支持 `gpt-5.4-high` 等 Codex CLI 模型别名，并保留 `max_completion_tokens`、`reasoning_effort`、tools / function calling 等常用参数。
- **Codex Free 额度展示**：Free 账号按实际周额度显示，不再把不存在的 5h 窗口展示为空卡片。
- **Claude Code 模型选择**：一键配置支持选择最多 4 个 Claude 模型槽位，可混合 DeepSeek、MiMo、Kimi、GLM。

## 快速开始

### 下载使用

1. 从 [GitHub Releases](https://github.com/mibgb65-cloud/OmniProxy/releases) 下载 Windows 安装包。
2. 启动 OmniProxy，在「账号管理」添加至少一个上游账号。
3. 在「全局设置」确认代理端口和厂商 Base URL。
4. 启动本地代理。
5. 将客户端 Base URL 指向 `http://127.0.0.1:3000`，或使用「一键配置」写入本地客户端配置。

### 从源码运行

依赖：

- Go
- Node.js
- Wails v2 CLI

```powershell
cd .\OmniProxyBackend
C:\Users\mimanchi\go\bin\wails.exe dev
```

或使用仓库脚本：

```powershell
.\scripts\dev.ps1
```

## 本地入口

| 协议 / 客户端 | 正式版地址 | Dev 版地址 |
| --- | --- | --- |
| OpenAI compatible | `http://127.0.0.1:3000` | `http://127.0.0.1:3001` |
| Codex backend | `http://127.0.0.1:3000/backend-api/codex` | `http://127.0.0.1:3001/backend-api/codex` |
| Codex Chat Completions | `http://127.0.0.1:3000/codex/v1` | `http://127.0.0.1:3001/codex/v1` |
| Claude router | `http://127.0.0.1:3000/anthropic-router` | `http://127.0.0.1:3001/anthropic-router` |
| Pi router | `http://127.0.0.1:3000/pi-router/v1` | `http://127.0.0.1:3001/pi-router/v1` |
| TokenRouter | `http://127.0.0.1:3000/tokenrouter/v1` | `http://127.0.0.1:3001/tokenrouter/v1` |
| Control API | `http://127.0.0.1:3890/api` | `http://127.0.0.1:3891/api` |

默认数据目录：

| 版本 | 数据目录 | 指针文件 |
| --- | --- | --- |
| 正式版 | `%USERPROFILE%\.omniproxy` | `%USERPROFILE%\.omniproxy-bootstrap.json` |
| Dev 版 | `%USERPROFILE%\.omniproxy-dev` | `%USERPROFILE%\.omniproxy-dev-bootstrap.json` |

## 支持矩阵

| 厂商 | 凭据类型 | 主要能力 |
| --- | --- | --- |
| OpenAI | API Key | OpenAI 兼容请求、rate-limit header 余量记录。 |
| OpenAI / Codex | `auth.json` | 自动解析邮箱、access token、account id，刷新 Codex 订阅额度，支持 Codex Responses 与 Chat Completions 兼容转换。 |
| Anthropic | API Key | Anthropic 原生请求和 Claude Code 路由。 |
| Anthropic / Claude | OAuth JSON | 支持 `access_token` / `refresh_token` 的 Claude OAuth JSON。 |
| DeepSeek | API Key | OpenAI 兼容入口和 Anthropic router。 |
| Kimi | API Key | Kimi Code 相关路由和订阅用量刷新。 |
| Xiaomi MiMo | API Key | 按量 API Key，通常以 `sk-` 开头。 |
| Xiaomi MiMo | Token Plan | Token Plan Key，通常以 `tp-` 开头，支持订阅额度展示。 |
| Zhipu GLM | API Key / Coding Plan | OpenAI 兼容、Anthropic router、Coding Plan 用量刷新。 |
| MiniMax | API Key | OpenAI 兼容入口和 Anthropic router。 |
| Gemini | API Key | Gemini API 路由和 Gemini CLI 一键配置。 |
| OpenRouter | API Key | 模型列表、余额查询、桌面端对话。 |
| TokenRouter | API Key | OpenAI 兼容路由，API Key 通常以 `tr_` 开头。 |
| 自定义网关 | API Key | OpenAI / Anthropic 兼容网关。 |

## 客户端一键配置

| 客户端 | 支持内容 |
| --- | --- |
| Codex | 写入本地 Codex backend 代理地址，支持恢复备份。 |
| Claude Code | 写入 Anthropic router，可选择 DeepSeek / MiMo / Kimi / GLM 模型槽位。 |
| Gemini CLI | 写入 Gemini 本地代理配置。 |
| OpenCode | 写入本地 provider 配置，支持 TokenRouter auto 模型。 |
| Pi Coding Agent | 写入单 provider 配置，统一通过 `/pi-router/v1` 按模型分流。 |

## 控制 API

桌面前端优先通过 Wails 绑定调用后端。HTTP 控制 API 保留给本地脚本和调试工具使用。除 `GET /api/control-token` 外，其它接口需要携带 `X-OmniProxy-Control-Token`，也支持 `Authorization: Bearer <token>`。

常用端点：

| 类型 | 端点 |
| --- | --- |
| 账号 | `GET /api/tokens`、`POST /api/tokens`、`PUT /api/tokens/{id}`、`DELETE /api/tokens/{id}` |
| 调度 | `PUT /api/tokens/{id}/selected`、`PUT /api/tokens/{id}/exclusive`、`DELETE /api/tokens/{id}/exclusive` |
| 验证 | `POST /api/tokens/{id}/validate` |
| 代理 | `GET /api/proxy/status`、`POST /api/proxy/start`、`POST /api/proxy/stop`、`GET /api/proxy/active-requests` |
| 配置 | `GET /api/config`、`PUT /api/config`、`GET /api/data-directory`、`PUT /api/data-directory` |
| 历史 | `GET /api/logs`、`GET /api/history`、`POST /api/history/clear` |
| 账单 | `GET /api/billing/usage`、`GET /api/billing/dates`、`POST /api/billing/clear` |
| 客户端配置 | `POST /api/codex/configure`、`POST /api/claude/models/configure`、`POST /api/opencode/configure`、`POST /api/pi/configure` |
| 更新 | `POST /api/update/check`、`POST /api/update/download`、`GET /api/update/download/status`、`POST /api/update/install` |

`/selected` 用于把账号加入或移出所属厂商的调度选择集合。没有已选账号时，调度器默认轮换该厂商全部可用账号；存在已选账号时，只在已选账号内轮换。

## 开发与验证

```powershell
cd .\OmniProxyBackend
go test ./...
```

```powershell
cd .\OmniProxyBackend\frontend
npm test
npm run build
```

正式构建：

```powershell
cd .\OmniProxyBackend
C:\Users\mimanchi\go\bin\wails.exe build
```

可与正式版共存的 Dev 构建：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build-dev.ps1 -Clean
```

Dev 版使用 `omniproxy_dev` build tag，应用标题、单实例 ID、数据目录和默认端口都与正式版隔离，适合在已安装正式版的机器上并行验证。

## 项目结构

```text
.
├── OmniProxyBackend/              # Wails 桌面主工程与 Go 后端
│   ├── internal/config/           # 本地配置、数据目录、默认值
│   ├── internal/logs/             # 请求和诊断日志
│   ├── internal/proxy/            # 代理、路由、鉴权、用量解析、WebSocket
│   ├── internal/storage/          # JSON / SQLite 本地持久化
│   ├── internal/token/            # 账号模型、Token 池、调度、额度状态
│   └── frontend/                  # Vue 3 + Vite + Element Plus 前端
├── docs/releases/                 # 人工整理的发布说明
├── scripts/dev.ps1                # 桌面开发启动脚本
├── scripts/build-dev.ps1          # 可共存 Dev exe 构建脚本
├── README.md                      # 中文文档
└── README_EN.md                   # English README
```

## 发布通道

| 通道 | Tag 示例 | GitHub Release 行为 |
| --- | --- | --- |
| Stable | `v1.1.2` | 正式 Release，适合日常使用。 |
| Beta | `v1.1.3-beta.1` | Pre-release，适合验证新功能和回归修复。 |
| Dev | `dev-*` | 本地构建版本，不作为公开 Release。 |

发布说明位于 `docs/releases/`。Beta 版本会标记为 GitHub Pre-release，正式版保留给稳定发布。

## 安全模型

- 默认只绑定 `127.0.0.1`，不面向公网或局域网暴露。
- 控制 API 使用本地控制令牌保护，桌面端自动获取和携带。
- Windows 上账号凭据使用当前用户 DPAPI 加密写入本地数据目录。
- 导出的账号池备份、Codex `auth.json` 和客户端配置备份可能包含真实凭据，请只保存到可信目录。
- 分享日志、截图或 Issue 前，请检查账号名、路径、请求 ID、Base URL 和 provider metadata。

## 路线图

- 更细粒度的额度趋势图和跨厂商对比视图。
- 更完整的 SSE、WebSocket、并发调度和异常恢复测试。
- 更多厂商、更多客户端工具和更多协议适配。
- 更严格的控制 API 本地访问边界。
- 更清晰的前端组件边界和可维护的设计系统。

## 贡献

欢迎提交 Issue 和 Pull Request。一个高质量问题报告通常包含：

- 操作系统、OmniProxy 版本和运行方式。
- 使用的客户端工具，例如 Codex、Claude Code、OpenCode、Pi Coding Agent 或自定义 API 客户端。
- 相关 provider、路由路径、模型名和错误日志。
- 预期行为、实际行为以及最小复现步骤。

提交 PR 前建议至少运行：

```powershell
cd .\OmniProxyBackend
go test ./...
```

```powershell
cd .\OmniProxyBackend\frontend
npm test
npm run build
```

## Star

如果 OmniProxy 改善了你的本地 AI 开发流程，欢迎点一个 Star。真实使用场景里的问题反馈、配置样例和回归用例，比泛泛的路线图更有价值。
