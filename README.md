# OmniProxy

OmniProxy 是一个本地 AI API 令牌调度与无感代理网关。当前仓库已实现一个 Wails 桌面应用闭环：Go 后端提供本地代理与控制 API，Vue 前端内嵌在桌面窗口中提供账号管理、日志、设置和手动验证界面。

## 当前架构

- `OmniProxyBackend/`：Wails 桌面主工程与 Go 后端。
  - 控制 API：默认 `http://127.0.0.1:3890/api`
  - 代理服务：默认 `http://127.0.0.1:3000`
  - 本地数据：默认写入 `%USERPROFILE%\.omniproxy`
- `OmniProxyBackend/frontend/`：桌面应用内嵌的 Vue 3 + Vite 前端。
- `omniproxyfrontend/`：早期分离式前端副本，后续可移除或合并。
- `scripts/dev.ps1`：旧的分离式开发启动脚本，当前桌面开发优先使用 Wails。

## 已完成能力

- Token 新增、编辑、删除、列表展示。
- Token 名称唯一校验，新账号默认置顶。
- JSON 本地持久化。
- 本地代理自动替换上游鉴权头。
- `429`、`502`、`503`、`504` 自动切换 Token 重试。
- 响应头额度解析：`x-ratelimit-remaining-tokens`、`x-ratelimit-remaining`、`x-ratelimit-remaining-requests`。
- 额度低于阈值自动标记为低额度。
- Token 手动验证，OpenAI/Codex 使用 `Authorization: Bearer`，Anthropic/Claude 使用 `x-api-key`。
- 账号管理页按 OpenAI、Anthropic、DeepSeek、Kimi、Xiaomi 分组。
- OpenAI 分组支持 API Key 和 Codex `auth.json` 两种凭据；Codex `auth.json` 兼容 `tokens.access_token` 与 `tokens.account_id`。
- Anthropic、DeepSeek、Kimi 当前只支持 API Key；Xiaomi 支持按量 API Key 和 Token Plan。
- 实时日志与代理启停。
- 保存代理端口、上游地址、重试次数后，运行中的代理会自动重启以加载新配置。
- 账号调度支持队列模式与优先平衡使用模式。
- Codex WebSocket 代理可在设置页开启或关闭，开启时会继续记录 WebSocket 请求用量。

## 开发运行

```powershell
cd .\OmniProxyBackend
C:\Users\mimanchi\go\bin\wails.exe dev
```

构建桌面应用：

```powershell
cd .\OmniProxyBackend
C:\Users\mimanchi\go\bin\wails.exe build
```

构建结果：

```powershell
.\OmniProxyBackend\build\bin\OmniProxy.exe
```

## 验证命令

```powershell
cd .\OmniProxyBackend
go test ./...
```

```powershell
cd .\OmniProxyBackend\frontend
npm run build
```

## 控制 API

- `GET /api/tokens`
- `POST /api/tokens`
- `PUT /api/tokens/{id}`
- `DELETE /api/tokens/{id}`
- `POST /api/tokens/{id}/validate`
- `GET /api/config`
- `PUT /api/config`
- `GET /api/logs`
- `GET /api/proxy/status`
- `POST /api/proxy/start`
- `POST /api/proxy/stop`

## 下一步

- 将当前控制 API 方法逐步迁移为 Wails 绑定，减少桌面窗口内的 HTTP 依赖。
- 引入 Pinia 和 UI 组件库，替换当前手写状态管理。
- 增加请求历史图表和额度趋势。
- 增加更完整的多厂商路由与鉴权策略。
- 增加 SSE 流式响应的端到端测试。
