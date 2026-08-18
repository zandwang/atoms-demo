# Atoms Demo 开发进度台账

> 本文件是项目开发状态的唯一持续记录。每次开始开发前先阅读；每次结束开发前必须更新“当前状态”和“开发日志”。

## 当前状态

| 项目 | 状态 | 说明 |
| --- | --- | --- |
| 最后更新 | 2026-08-18 | M3/M4 代码闭环完成，进入本地验收准备 |
| 当前阶段 | M3/M4 集成与 M5 本地验收 | 进行中 |
| 当前阻塞项 | `.env` 尚缺 endpoint 支持的 `OPENAI_MODEL`，因此尚未执行真实模型联调；`go mod tidy` 的额外测试依赖受外部镜像网络超时影响 | 现有锁定依赖的构建、测试和二进制运行不受影响 |
| 下一步 | 补充 `OPENAI_MODEL` 后执行一次真实生成、预览交互和刷新/重启验收 | 详见本条目最后的“下次从这里继续” |
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
- [x] 实现 `todo` 模板的 Go 编译器，生成 HTML/CSS/JS artifact。
- [x] 实现对话时间线、提示词输入、真实阶段反馈、错误和重试交互。
- [x] 实现受 sandbox 限制的 iframe 预览和代码查看。
- [ ] 使用真实 API 完成一次“输入 → 待办应用预览”的本地联调（等待 `OPENAI_MODEL`）。

完成条件：用户可输入需求，看到真实模型结果、可运行待办应用及编译代码；失败不丢失请求且可重试。

### M4：模板扩展、迭代与版本历史

状态：`进行中`

- [x] 实现 `notes` 与 `habits` 编译器和对应规格校验。
- [x] 实现基于当前版本的自然语言迭代；新结果生成不可变版本。
- [x] 实现版本列表、版本详情、激活/回滚和 system message。
- [x] 实现 iframe `postMessage` 状态桥接与 `preview_states` API。
- [x] 通过 SQLite 重开、版本激活和状态 API 测试验证预览内数据、活跃版本和项目记录可恢复。

完成条件：三类应用可稳定生成；用户可迭代、查看历史、回滚，并恢复每个版本的预览运行态。

### M5：质量、体验与本地验收

状态：`进行中`

- [x] 完成响应式三栏 UI、加载/空/错误状态和基本可访问性。
- [ ] 补齐 Go 单元/集成测试、React 组件测试和 Playwright 主流程测试（Go 覆盖已完成，前端/E2E 待补）。
- [x] 覆盖模型超时、无效输出、配置错误、项目归属和预览状态异常。
- [x] 编写 README：依赖、环境变量、启动、构建、测试、演示步骤和已知限制。
- [ ] 按 [需求分析计划](./REQUIREMENTS_ANALYSIS_PLAN.md) 的端到端验收场景逐条验收。

完成条件：所有 P0、选定的 P1（版本历史/回滚）和本地验收场景通过；交付物可由他人依文档启动。

### M6：线上部署（当前不在范围内）

状态：`不做`

原因：用户先验收本地 Demo，再决定部署平台、公开访问和跨设备需求。

恢复条件：用户确认要上线后，补充部署平台、域名/访问策略、数据库持久卷和 API Key 注入方案。

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
