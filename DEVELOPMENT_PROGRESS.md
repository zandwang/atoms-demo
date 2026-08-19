# Atoms Demo 开发进度台账

> 本文件是项目开发状态的唯一持续记录。每次开始开发前先阅读；每次结束开发前必须更新“当前状态”和“开发日志”。

## 当前状态

| 项目 | 状态 | 说明 |
| --- | --- | --- |
| 最后更新 | 2026-08-19 | 已恢复 `.env` 默认模型配置，并完成安全的用户覆盖优先级 |
| 当前阶段 | M3/M4 通用生成、迭代与兼容收尾 | 进行中 |
| 当前阻塞项 | 无 | 正在恢复服务端默认模型配置，并保留用户覆盖能力 |
| 下一步 | 使用默认配置和用户覆盖各完成一次真实生成验收，再提交部署 | 先保持历史 SQLite 版本可读取 |
| 本地运行目标 | Go 单二进制 | React 仅为构建期依赖 |

## 状态标记

| 标记 | 含义 |
| --- | --- |
| `未开始` | 尚未执行，未产生代码或配置变更 |
| `进行中` | 已开始但未满足该任务的完成条件 |
| `已完成` | 已实现并完成本任务要求的验证 |
| `阻塞` | 无法继续，必须注明原因、影响和解除条件 |
| `不做` | 经确认不在本阶段范围，必须注明决策原因 |

## 里程碑计划

### M0：需求与设计

状态：`已完成`

- [x] 提取 `requirement.txt` 的 P0/P1、验收项与风险。
- [x] 完成 [需求分析计划](./REQUIREMENTS_ANALYSIS_PLAN.md)。
- [x] 确认真实模型、小型单页应用范围、轻量初始化、版本回滚和本地优先交付。
- [x] 完成 [Go 技术设计](./CODE_DESIGN.md)。

完成条件：需求、范围、架构、数据、接口、安全边界和验收路径均可追溯。

### M1：Go 工程骨架与本地启动

状态：`已完成`

- [x] 初始化 `go.mod`（目标 Go 1.26）和 Go 目录结构。
- [x] 初始化 `web/` 的 React + TypeScript + Vite 工程。
- [x] 配置 Vite 输出到 `internal/webembed/dist`，并由 `//go:embed` 提供静态资源。
- [x] 增加 `.gitignore`、`.env.example`、`Makefile` 与最小 README。
- [x] 实现配置读取、结构化日志、健康检查和 HTTP server 启动。
- [x] 让未配置模型时返回可行动的 `CONFIG_ERROR`，且不泄露敏感值。

完成条件：`make build` 能生成单个 Go 可执行程序；启动后可访问页面和健康检查；不配置模型也能得到清晰提示。

### M2：SQLite、会话与项目工作区

状态：`已完成`

- [x] 实现嵌入式 SQLite migrations、WAL、foreign keys 和临时数据库测试工具。
- [x] 实现匿名 workspace 与 HttpOnly session Cookie。
- [x] 实现初始化、会话读取、项目创建/查询/重命名/删除 API。
- [x] 实现 React Welcome、项目侧栏和空状态。
- [x] 验证重启 Go 服务和刷新浏览器后，昵称与项目仍可恢复。

完成条件：一名本地用户可以初始化、创建、打开、修改和删除项目；每个 API 都按 workspace 隔离数据。

### M3：Agent 生成最短闭环

状态：`进行中`

- [x] 定义 `AppSpec`、`AgentResult`、错误码及所有输入/输出验证。
- [x] 实现 OpenAI-compatible model adapter、超时、错误映射和 fake adapter。
- [x] 实现 Agent prompt builder 与 `POST /api/projects/{projectID}/generate` SSE。
- [x] 实现通用 HTML/CSS/JS 文件生成器，替换新生成链路中的固定模板选择。
- [x] 实现对话时间线、提示词输入、真实阶段反馈、错误和重试交互。
- [x] 实现受 sandbox 限制的 iframe 预览和代码查看。
- [ ] 使用用户提供的 endpoint、model、Key 完成一次“输入 → 任意小型 SPA 预览”的本地联调。

完成条件：用户可输入需求，看到真实模型结果、可运行的小型单页应用及生成文件；失败不丢失请求且可重试。

### M4：通用迭代与版本历史

状态：`进行中`

- [x] 移除三模板作为产品边界，加入通用文件版本和安全校验；旧模板仅保留历史读取兼容。
- [x] 实现基于当前版本的自然语言迭代；新结果生成不可变版本。
- [x] 实现版本列表、版本详情、激活/回滚和 system message。
- [x] 实现 iframe `postMessage` 状态桥接与 `preview_states` API。
- [x] 通过 SQLite 重开、版本激活和状态 API 测试验证预览内数据、活跃版本和项目记录可恢复。

完成条件：受约束的小型单页应用可稳定生成；用户可迭代、查看历史、回滚，并恢复每个版本的预览运行态。

### M5：质量、体验与本地验收

状态：`进行中`

- [x] 完成响应式三栏 UI、加载/空/错误状态和基本可访问性。
- [ ] 补齐 React 组件测试和 Playwright 主流程测试（Go 单元/集成覆盖已完成）。
- [x] 覆盖模型超时、无效输出、配置错误、项目归属和预览状态异常。
- [x] 编写 README：依赖、环境变量、启动、构建、测试、演示步骤和已知限制。
- [ ] 按 [需求分析计划](./REQUIREMENTS_ANALYSIS_PLAN.md) 的端到端验收场景逐条验收。
- [x] 支持 `.env` 默认模型认证和用户覆盖：用户 Key 仅在标签页和单次服务端请求内存中存在。

完成条件：所有 P0、选定的 P1（版本历史/回滚）和本地验收场景通过；交付物可由他人依文档启动。

### M6：线上部署

状态：`进行中`

原因：用户已选择 Render 进行小规模在线验收；当前先处理构建、端口和健康检查，暂不承诺正式账号和持久化数据。

后续条件：补充持久化数据库、真实账号、限流和公开服务成本控制后，才可评估正式多人使用。

## 开发开始/恢复清单

每次开始或恢复开发时，按以下顺序执行：

1. 阅读本文件的“当前状态”、当前里程碑和最后一条开发日志。
2. 阅读相关设计文档，并确认当前任务未超出 [CODE_DESIGN.md](./CODE_DESIGN.md) 的范围。
3. 检查工作区变更和已有文件，不覆盖未确认的用户修改。
4. 执行与当前阶段相关的最小验证（例如构建、单元测试或手工主流程）。
5. 选择当前里程碑中最小的未完成任务；开始时将其标记为 `进行中`。
6. 若发现架构、范围、API 或数据模型必须改变，先更新设计文档和本台账，再继续编码。

## 开发结束更新协议

每次结束一次包含实现、配置或测试工作的开发会话前，必须完成：

1. 更新“当前状态”：最后更新日期、当前阶段、阻塞项和下一步。
2. 更新相应里程碑：勾选已完成项；为进行中或阻塞项写明具体状态。
3. 在“开发日志”添加一条记录，包含文件、验证命令及结果、未完成事项和明确下一步。
4. 若实际实现偏离设计，先同步更新 `CODE_DESIGN.md`，并在日志注明决策。
5. 不记录 `.env` 的值、API Key、Cookie、令牌或其他敏感数据。

仅进行纯讨论且未改动项目时，可以不新增日志；一旦开始代码、配置、文档设计或测试工作，则在结束时更新。

### 开发日志条目模板

```md
### YYYY-MM-DD — <阶段/任务> — <状态>

- 完成：
  - …
- 修改：
  - `path/to/file` — …
- 验证：
  - `<command>` — 通过 / 失败（原因）
- 未完成或风险：
  - …
- 下次从这里继续：
  1. …
```

## 开发日志

### 2026-08-19 — `.env` 默认模型与安全覆盖 — 已完成

- 完成：
  - 恢复 `OPENAI_BASE_URL`、`OPENAI_MODEL`、`OPENAI_API_KEY` 的服务端默认配置读取。
  - 浏览器无配置时使用完整默认值；只填写 Key 或 model 时按字段覆盖默认值。
  - 浏览器填写 endpoint 时强制要求用户同时提供 model 和 Key，禁止默认 Key 流向用户控制的 endpoint。
  - 前端健康状态、生成按钮和模型配置提示改为识别默认配置与部分覆盖状态。
- 修改：
  - `internal/config/config.go`、`internal/config/config_test.go` — 默认模型字段、健康布尔状态和读取测试。
  - `internal/app/generation.go`、`internal/app/server_test.go` — 配置解析和安全覆盖矩阵测试。
  - `web/src/App.tsx` — 默认模型可用性判断、覆盖配置提示和状态徽标。
  - `README.md`、`CODE_DESIGN.md`、`REQUIREMENTS_ANALYSIS_PLAN.md` — 同步默认配置与安全边界说明。
- 验证：
  - `GOCACHE=/private/tmp/atoms-demo-go-build-cache go test ./...` — 通过。
  - `GOCACHE=/private/tmp/atoms-demo-go-build-cache go test -race ./...` — 通过。
  - `GOCACHE=/private/tmp/atoms-demo-go-build-cache go vet ./...`、`go mod verify`、`git diff --check` — 通过。
  - `cd web && npm run build` — 通过。
  - `GOCACHE=/private/tmp/atoms-demo-go-build-cache make build` — 通过；Go module stat cache 有非阻断权限警告。
  - 未记录 `.env` 实际值、API Key、Cookie 或令牌。
- 未完成或风险：
  - 尚未使用真实默认配置和真实用户覆盖配置各完成一次端到端生成验收。
  - 公开部署使用共享默认 Key 时仍需限流、额度和滥用控制。
- 下次从这里继续：
  1. 启动服务，验证无浏览器配置、只填 Key、完整自定义 endpoint 三种生成路径。
  2. 通过真实模型验收后提交并重新部署。

### 2026-08-19 — 生成过程可视化 — 已完成

- 完成：
  - 将“正在请求模型…”单行提示升级为五步进度时间线：准备上下文、生成应用文件、校验文件、编译预览、保存版本。
  - 进度面板显示当前需求摘要、真实耗时、当前/完成/等待状态，不使用虚假百分比或模型隐藏思维链。
  - SSE 新增 `preparing_context` 和 `saving_version` 两个真实服务端阶段；成功后继续展示 Agent plan、完整回复、生成文件和预览。
- 修改：
  - `internal/app/generation.go`、`internal/app/server_test.go` — 新阶段事件和 SSE 覆盖。
  - `web/src/api/client.ts`、`web/src/App.tsx` — 阶段类型、累计状态和响应式进度面板。
  - `CODE_DESIGN.md` — 明确阶段协议与不展示隐藏思维链的边界。
- 验证：
  - `GOCACHE=/private/tmp/atoms-demo-go-build-cache go test ./internal/app` — 通过。
  - `cd web && npm run build` — 通过。
  - `git diff --check` — 通过。
- 未完成或风险：
  - OpenAI-compatible adapter 当前使用非流式 JSON 响应，因此模型生成阶段无法安全展示部分回复；完整回复在校验和保存成功后展示。
- 下次从这里继续：
  1. 重启 `make run`，观察一次约 30–120 秒生成过程中的计时和阶段切换。
  2. 确认成功后 Agent 回复、计划、代码和预览正常刷新。

### 2026-08-19 — 右侧预览区扩展 — 已完成

- 完成：
  - 将桌面端右侧预览栏宽度从最大 420px 调整为 440–560px。
  - 将预览 iframe 桌面最小高度从 430px 提升到 620px，并按视口高度增长；移动端使用 520px 稳定高度，避免小屏布局异常。
  - 保留代码、文件和版本列表的内部滚动，不让生成文件内容撑坏整个工作台。
- 修改：
  - `web/src/App.tsx` — 三栏网格、预览面板和 iframe 响应式尺寸。
- 验证：
  - `cd web && npm run build` — 通过。
  - `git diff --check` — 通过。
- 未完成或风险：
  - 尚未在用户实际浏览器尺寸下截图验收；极窄桌面窗口会自动进入移动单列布局。
- 下次从这里继续：
  1. 刷新浏览器验证右栏宽度和 iframe 高度。
  2. 继续真实模型生成验收。

### 2026-08-19 — 模型生成诊断日志与可配置超时 — 已完成

- 完成：
  - 为生成任务增加开始、模型调用开始/完成/失败、校验/编译失败和最终完成/失败日志。
  - 日志包含 project/attempt/version ID、阶段耗时、45 秒超时配置、错误码、上游 HTTP 状态、消息数量和生成文件大小。
  - 对网络根因进行脱敏归类；不记录用户需求正文、endpoint、model、API Key、Authorization、Cookie 或上游响应体。
  - 模型 adapter 保留安全的上游 HTTP 状态用于诊断，并修正旧三模板的过期错误文案。
  - 根据真实日志确认 45 秒写死超时导致通用应用生成失败；改为默认 120 秒，并支持 `ATOMS_MODEL_TIMEOUT` 在 10 秒到 10 分钟之间配置。
  - 保留 `context.WithTimeoutCause` 的自定义原因，日志显示 `model generation deadline exceeded`，不再只显示 `type=*errors.errorString`。
- 修改：
  - `internal/app/generation.go` — 生成链路结构化日志和网络原因脱敏。
  - `internal/config/config.go`、`.env.example`、`cmd/atoms-demo/main.go` — 模型超时配置、范围校验和启动日志。
  - `internal/agent/adapter.go`、`internal/agent/openai_compatible.go` — 安全上游状态和通用范围错误文案。
  - `internal/app/server_test.go`、`internal/agent/openai_compatible_test.go` — 日志脱敏、超时日志和 HTTP 状态测试。
  - `internal/config/config_test.go` — 默认值、有效值和非法超时配置测试。
  - `README.md`、`CODE_DESIGN.md` — 排查方式与日志安全边界。
- 验证：
  - `GOCACHE=/private/tmp/atoms-demo-go-build-cache go test ./...` — 通过。
  - `GOCACHE=/private/tmp/atoms-demo-go-build-cache go test -race ./...` — 通过。
  - `GOCACHE=/private/tmp/atoms-demo-go-build-cache go vet ./...` — 通过。
  - `go mod verify`、`GOCACHE=/private/tmp/atoms-demo-go-build-cache make build`、`git diff --check` — 通过；本机 Go module stat cache 仍有非阻断权限警告。
- 未完成或风险：
  - 尚未用用户的真实模型 endpoint 在 120 秒配置下完成成功生成验收；如果仍超时，需根据新的 `attempt_id` 和 `cause` 继续区分模型服务或网络问题。
- 下次从这里继续：
  1. 重启 `make run` 后再次生成，确认启动日志为 `model_timeout=2m0s`，找到同一 `attempt_id` 的模型阶段日志。
  2. 若仍超时，检查新的 `cause` 是否为 `model generation deadline exceeded`，或出现 DNS/网络错误。
  3. 验收成功后提交、推送并重新部署 Render。

### 2026-08-19 — 目标纠偏：通用小型单页应用生成 — 首轮完成

- 完成：
  - 根据用户确认，将产品目标从“三种固定模板生成器”改为“用户描述任意小型单页应用，AI 生成可操作网页并支持自然语言迭代”。
  - 更新需求分析和技术设计，明确受约束的 `index.html`、`styles.css`、`app.js` 文件集合、代码安全边界和 sandbox 预览。
- 修改：
  - `REQUIREMENTS_ANALYSIS_PLAN.md` — 将生成范围改为通用小型单页应用。
  - `CODE_DESIGN.md` — 将 AppSpec/固定编译器方向改为受约束文件生成。
  - `DEVELOPMENT_PROGRESS.md` — 标记核心生成链路重构开始。
- 验证：
  - 文档范围、用户旅程和 Definition of Ready 已更新；尚未开始代码重构。
- 未完成或风险：
  - 当前代码仍是 todo/notes/habits 三模板实现，不能作为最终目标交付。
  - 已完成严格文件校验、危险 API 检查、通用预览状态和迭代上下文限制；仍需真实模型回归和线上部署后验收。
- 下次从这里继续：
  1. 使用真实 endpoint、model、Key 完成一次通用应用生成和自然语言迭代。
  2. 提交并推送本次改动，触发 Render 重新部署。
  3. 视历史数据迁移需要，再决定是否删除旧模板兼容编译器。

### 2026-08-19 — 通用生成链路实现与验证 — 已完成

- 完成：
  - 新增 `custom` AppSpec，固定为 `indexHtml`、`stylesCss`、`appJs` 三个文件；新 Agent prompt 只生成该协议。
  - 增加文件大小、NUL、外部资源、网络 API、Cookie、父窗口导航和内联容器逃逸校验；通用预览状态允许受限 JSON object。
  - 新增通用 compiler，注入 CSP 和 `atomsPreview` 状态桥接；旧 todo/notes/habits 代码只作为历史 SQLite 兼容路径保留。
  - 将前端引导、README、需求分析和技术设计改为任意小型单页应用目标，并明确 Demo 级 sandbox 限制。
  - 将 domain、agent、compiler、store 和 app 主生成测试数据切换为番茄钟 custom 应用，覆盖生成、版本、回滚和状态恢复链路。
- 修改：
  - `internal/domain/generation.go`、`internal/compiler/generic.go`、`internal/store/sqlite/generations.go`、`internal/agent/prompt.go` — 通用文件协议、校验、编译、持久化和模型上下文。
  - `internal/domain/generation_test.go`、`internal/compiler/generic_test.go`、`internal/agent/openai_compatible_test.go`、`internal/store/sqlite/store_test.go`、`internal/app/server_test.go` — 通用协议及端到端 fake 覆盖。
  - `web/src/App.tsx`、`README.md`、`REQUIREMENTS_ANALYSIS_PLAN.md`、`CODE_DESIGN.md` — 产品文案、范围、安全边界和验收描述。
- 验证：
  - `GOCACHE=/private/tmp/atoms-demo-go-build-cache go test ./...` — 通过。
  - `GOCACHE=/private/tmp/atoms-demo-go-build-cache go test -race ./...` — 通过。
  - `GOCACHE=/private/tmp/atoms-demo-go-build-cache go vet ./...` — 通过。
  - `go mod verify` — 通过。
  - `cd web && npm run build` — 通过。
  - `GOCACHE=/private/tmp/atoms-demo-go-build-cache make build` — 通过；默认本机 GOCACHE 的权限警告不影响代码构建。
  - `git diff --check` — 通过。
- 未完成或风险：
  - 尚未使用真实用户模型配置完成新协议的浏览器端到端验收。
  - 旧模板兼容编译器和 schema 尚未迁移删除；公开部署仍需限流、资源配额和更强的生成代码审查。
- 下次从这里继续：
  1. 用真实模型生成番茄钟，继续要求“改成 50 分钟并增加今日完成次数”，检查版本和回滚。
  2. 提交、推送并重新部署 Render，验证线上 `make build` 和 BYOK。
  3. 根据是否需要保留历史数据库决定旧模板迁移方案。

### 2026-08-19 — M6 Render 构建适配 — 已完成

- 完成：
  - 定位 Render 构建失败根因：干净构建环境没有安装 `web/node_modules`，TypeScript 因缺少 React/Vite 类型产生大量连锁 JSX 错误。
  - 让 `make build` 自动依赖 `web-install`，在 Render 等干净环境先执行 `npm ci`。
  - 增加平台 `PORT` 适配：未设置 `ATOMS_ADDR` 时自动监听 `:${PORT}`。
- 修改：
  - `Makefile` — `web-build` 依赖 `web-install`。
  - `internal/config/config.go`、`internal/config/config_test.go` — 读取 `PORT` 并增加配置测试。
  - `README.md` — 记录平台端口行为。
- 验证：
  - `GOCACHE=/private/tmp/atoms-demo-go-build-cache make build` — 通过（本机 Go module stat cache 有非阻断权限警告）。
  - `GOCACHE=/private/tmp/atoms-demo-go-build-cache go test ./...` — 通过。
  - `GOCACHE=/private/tmp/atoms-demo-go-build-cache go test -race ./...` — 通过。
  - `GOCACHE=/private/tmp/atoms-demo-go-build-cache go vet ./...`、`go mod verify`、`git diff --check` — 通过。
- 未完成或风险：
  - 尚未重新触发 Render 部署；Render 原生 Go 环境需满足项目要求的 Go 版本。
  - 免费实例本地 SQLite 可能丢失，且没有公开服务限流。
- 下次从这里继续：
  1. 提交本次修改并在 Render 保持 Build Command 为 `make build`。
  2. 配置 Start Command 为 `./bin/atoms-demo`，Health Check Path 为 `/api/health`。
  3. 重新部署后进行页面、BYOK 和自定义 endpoint 真实生成验收。

### 2026-08-19 — M5 自定义模型 endpoint/model — 已完成

- 完成：
  - 将 BYOK 从仅用户 API Key 扩展为用户自定义 OpenAI-compatible endpoint、model 和 API Key。
  - 三项配置仅保存在当前标签页 `sessionStorage`，通过 `X-Model-Base-URL`、`X-Model-Name`、`X-Model-API-Key` 请求头发送。
  - 增加 endpoint URL 语法校验、DNS 地址策略、默认私网/回环拒绝、禁用代理访问和受信环境显式放行开关。
- 修改：
  - `internal/agent/endpoint.go`、`internal/agent/openai_compatible.go` — endpoint 校验、私网防护和请求级 adapter。
  - `internal/app/generation.go`、`internal/config/`、`cmd/atoms-demo/main.go` — 请求头配置、BYOK 健康状态和私网开关。
  - `web/src/App.tsx`、`web/src/api/client.ts` — endpoint/model/API Key 配置弹窗、标签页存储和请求头。
  - `CODE_DESIGN.md`、`README.md`、`.env.example` — 自定义模型配置契约和安全说明。
- 验证：
  - `GOCACHE=/private/tmp/atoms-demo-go-build-cache go test ./...` — 通过。
  - `GOCACHE=/private/tmp/atoms-demo-go-build-cache go test -race ./...` — 通过。
  - `GOCACHE=/private/tmp/atoms-demo-go-build-cache go vet ./...` — 通过。
  - `go mod verify`、`npm run build` — 通过。
  - endpoint 语法、私网默认拒绝/显式放行和缺少模型配置的测试 — 通过。
- 未完成或风险：
  - 真实浏览器自定义 endpoint 联调尚未完成；公开部署时不要启用私网 endpoint 放行。
  - 匿名 workspace、SQLite 持久化、限流和模型费用控制仍需部署阶段处理。
- 下次从这里继续：
  1. 在浏览器填写实际 endpoint、model 和 Key，完成一次真实生成验收。
  2. 选择部署平台并补充 `PORT`、持久化数据库和公开服务限流方案。

### 2026-08-18 — M5 BYOK 模型认证 — 已完成

- 完成：
  - 将模型认证从服务端共享 `OPENAI_API_KEY` 改为用户自带 Key（BYOK）。
  - 前端通过 `sessionStorage` 保存当前标签页的 Key，并在生成 SSE 请求中发送 `X-Model-API-Key`。
  - Go 服务端固定 `OPENAI_BASE_URL` 和 `OPENAI_MODEL`，按请求临时构造模型 adapter；缺少 Key 返回 `API_KEY_REQUIRED`。
  - 明确 Key 不写入 SQLite、Cookie、日志、SSE 响应或构建产物，并保留 endpoint/model 固定以避免任意代理与 SSRF 风险。
- 修改：
  - `internal/config/`、`internal/app/`、`internal/agent/` — 移除服务端 Key 依赖、增加请求头认证和安全错误映射。
  - `web/src/App.tsx`、`web/src/api/client.ts` — 增加 Key 设置/清除入口、标签页存储和 SSE 请求头。
  - `.env.example`、`README.md`、`CODE_DESIGN.md` — 更新 BYOK 配置契约、安全边界和使用说明。
- 验证：
  - `GOCACHE=/private/tmp/atoms-demo-go-build-cache go test ./...` — 通过。
  - `GOCACHE=/private/tmp/atoms-demo-go-build-cache go test -race ./...` — 通过。
  - `GOCACHE=/private/tmp/atoms-demo-go-build-cache go vet ./...` — 通过。
  - `go mod verify` — 通过。
  - `npm run build`（在 `web/`）— 通过。
  - `git diff --check`、`.env` 忽略规则检查 — 通过；未记录任何 Key 值。
- 未完成或风险：
  - 仍是匿名 workspace；清除 Cookie 后无法恢复工作区，尚未实现真实账号和跨设备登录。
  - SQLite 线上持久化、单实例约束、限流和模型成本控制需要在部署阶段处理。
  - React 组件测试、Playwright 主流程和真实浏览器 BYOK 验收仍待补充。
- 下次从这里继续：
  1. 在浏览器设置用户自己的 Key，完成一次真实生成并确认刷新/关闭标签页后的 Key 行为。
  2. 选择 Render/Koyeb/Cloud Run 等部署方式，补充 `PORT`、持久化磁盘或托管数据库方案。
  3. 若面向更大规模用户，增加真实账号、会话撤销、限流和额度控制。

### 2026-08-18 — M0 需求与设计 — 已完成

- 完成：
  - 基于 `requirement.txt` 完成需求分析、范围分层、验收与风险计划。
  - 确认模型使用方式、生成边界、轻量初始化、版本回滚和本地优先交付。
  - 将技术设计从 Node.js 后端调整为 Go 1.26 后端 + React 构建产物嵌入 Go 二进制。
  - 建立本开发进度台账及每次开发结束前的强制更新约定。
- 修改：
  - `REQUIREMENTS_ANALYSIS_PLAN.md` — 需求与已确认决策。
  - `CODE_DESIGN.md` — Go 后端、SQLite、SSE、静态嵌入、接口、测试设计。
  - `DEVELOPMENT_PROGRESS.md` — 当前状态、里程碑、恢复清单和开发日志。
  - `AGENTS.md` — 项目级进度更新约定。
  - `.env` — 仅规范变量名为 `OPENAI_BASE_URL`、`OPENAI_API_KEY`；值未记录。
- 验证：
  - `go version` — 本机为 Go 1.26.4。
  - `node --version` / `npm --version` — 本机具备前端构建环境。
  - 文档标题和过期 Node.js 后端设计检索 — 通过。
- 未完成或风险：
  - 尚未建立 `go.mod`、React 工程、SQLite schema 或任何应用代码。
  - 真实模型联调前需要在 `.env` 添加受该 API endpoint 支持的 `OPENAI_MODEL`。
- 下次从这里继续：
  1. 开始 M1：初始化 Go 1.26 工程、Go HTTP server 和基础目录。
  2. 创建 React/Vite 前端并验证静态产物可被 Go embed。
  3. 添加安全的 `.env.example`、`.gitignore`、构建脚本和最小 README。

### 2026-08-18 — M1/M2 工程、工作区与项目持久化 — 已完成

- 完成：
  - 建立 Go 1.26 服务、React/Vite 构建工程、静态资源嵌入、配置读取、健康检查和结构化日志。
  - 建立 SQLite migration、WAL、匿名工作区与 HttpOnly 会话；完成项目创建、查询、改名、删除和 workspace 隔离。
  - 完成 React 初始化页、项目侧栏、项目选择及 CRUD 界面。
- 修改：
  - `cmd/atoms-demo/main.go`、`internal/config/`、`internal/app/` — 本地 HTTP 服务、配置、会话与项目 API。
  - `internal/store/`、`internal/store/sqlite/`、`internal/store/migrations/` — SQLite schema、迁移与仓储实现。
  - `web/`、`internal/webembed/` — React 工作台、API 客户端与 Go embed 构建产物。
  - `Makefile`、`.env.example`、`.gitignore`、`README.md`、`go.mod`、`go.sum` — 工程、构建和使用说明。
- 验证：
  - `npm run build`（在 `web/`）— 通过。
  - `go test ./...` — 通过。
  - `make build` — 通过，生成 `bin/atoms-demo`。
  - 临时数据目录启动二进制后的 HTTP 冒烟 — 通过：首次初始化、项目创建/改名、第二工作区无法读取第一工作区项目、重启后会话和项目仍可恢复、嵌入式首页可访问。
- 未完成或风险：
  - `GOPROXY=https://proxy.golang.org,direct go mod tidy` 需要的若干间接测试依赖下载因外部网络超时未完成；当前 `go test ./...` 和 `make build` 均通过。
  - M3 真实模型联调前仍需提供 endpoint 支持的 `OPENAI_MODEL`；未记录或输出任何敏感配置值。
- 下次从这里继续：
  1. 在 `.env` 增加 endpoint 支持的 `OPENAI_MODEL`（不要把值写入日志或文档）。
  2. 启动 `make run`，完成一次真实模型生成及三类模板的人工预览验收。
  3. 在 iframe 中新增/完成/删除或编辑数据，刷新浏览器并重启服务，确认版本和运行态恢复。

### 2026-08-18 — M3/M4 受控生成、版本与预览运行态 — 进行中

- 完成：
  - 实现严格判别式 `AppSpec`、`AgentResult`、输入长度/枚举/字段校验及安全错误码。
  - 实现 OpenAI-compatible adapter（JSON mode 回退、超时、认证/上游错误映射）和返回隔离已校验结果的显式 fake adapter。
  - 实现真实阶段 SSE、请求/失败消息持久化、确定性编译器及三类可交互应用：`todo`、`notes`、`habits`。
  - 实现版本列表、不可变生成版本、自然语言迭代上下文、激活/回滚和 system message。
  - 实现 sandbox iframe 的 `postMessage` 状态桥接；服务端按模板校验并保存 `preview_states`，且避免晚到生成结果覆盖用户回滚的 active version。
- 修改：
  - `internal/domain/generation.go` — 三模板规格、Agent 结果、版本对象和预览状态校验。
  - `internal/agent/` — prompt、OpenAI-compatible adapter、fake adapter 和安全错误映射。
  - `internal/compiler/` — 固定模板编译、CSP、三类应用交互和预览桥接。
  - `internal/store/repository.go`、`internal/store/sqlite/generations.go` — 消息/版本/激活/预览状态事务与重开恢复。
  - `internal/app/` — SSE 生成、版本/状态 API 和 HTTP 错误映射。
  - `web/src/api/client.ts`、`web/src/App.tsx` — SSE 消费、Agent 时间线、预览/代码/规格、版本恢复和状态保存界面。
  - `CODE_DESIGN.md`、`README.md`、`go.mod` — 实际目录、当前能力、运行文档和直接 SQLite 依赖声明。
- 验证：
  - `GOCACHE=/private/tmp/atoms-demo-go-build-cache go test ./...` — 通过。
  - `GOCACHE=/private/tmp/atoms-demo-go-build-cache go test -race ./...` — 通过。
  - `GOCACHE=/private/tmp/atoms-demo-go-build-cache go vet ./...` — 通过。
  - `go mod verify` — 通过。
  - `npm run build`（在 `web/`）— 通过。
  - `GOCACHE=/private/tmp/atoms-demo-go-build-cache make build` — 通过，生成约 16 MB 的 arm64 Go 单二进制。
  - `httptest` 集成测试 — 通过 SSE 阶段顺序、版本激活/回滚、workspace 隔离、非法模型结果、三种模板的非法/合法预览状态和 SQLite 关闭/重开恢复。
  - 最终二进制临时目录冒烟 — 通过健康检查、嵌入式首页、会话初始化和项目 API；未输出 Cookie 或配置值。
- 未完成或风险：
  - `.env` 尚缺 `OPENAI_MODEL`，真实模型端点/模型能力和三类模板的人工联调尚未验证。
  - React 组件测试、Playwright 浏览器主流程和真实 iframe 手工验收尚未补齐；当前由 TypeScript/Vite 构建和 Go HTTP/存储集成测试兜底。
  - `GOPROXY=https://proxy.golang.org,direct go mod tidy` 的额外测试依赖下载仍因外部网络超时未完成；`go mod verify`、构建和测试均通过。
- 下次从这里继续：
  1. 补充并验证 `OPENAI_MODEL`，完成一次真实 API 生成。
  2. 运行浏览器主流程，操作三类预览并检查刷新/重启后的运行态。
  3. 如验收需要，再补 React 组件测试与 Playwright 脚本；完成后关闭 M3/M4/M5 的剩余勾选项。
