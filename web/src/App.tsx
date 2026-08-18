import { type FormEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";

import {
  APIRequestError,
  type GenerationEvent,
  type GenerationVersion,
  type Health,
  type Message,
  type Project,
  type Workspace,
  activateVersion,
  createProject,
  deleteProject,
  generateProject,
  getHealth,
  getPreviewState,
  getSession,
  initializeSession,
  listMessages,
  listProjects,
  listVersions,
  renameProject,
  savePreviewState
} from "./api/client";

type BootstrapState = "loading" | "ready" | "uninitialized" | "error";
type ProjectDataState = "idle" | "loading" | "ready" | "error";
type HealthState =
  | { kind: "loading" }
  | { kind: "ready"; value: Health }
  | { kind: "error" };
type GenerationState =
  | { kind: "idle" }
  | { kind: "running"; projectID: string; status: string; label: string }
  | { kind: "failed"; projectID: string; message: string; retryable: boolean };
type PreviewTab = "preview" | "code" | "spec";

export function App() {
  const [bootstrap, setBootstrap] = useState<BootstrapState>("loading");
  const [workspace, setWorkspace] = useState<Workspace | null>(null);
  const [projects, setProjects] = useState<Project[]>([]);
  const [selectedProjectID, setSelectedProjectID] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [versions, setVersions] = useState<GenerationVersion[]>([]);
  const [projectDataState, setProjectDataState] = useState<ProjectDataState>("idle");
  const [health, setHealth] = useState<HealthState>({ kind: "loading" });
  const [generation, setGeneration] = useState<GenerationState>({ kind: "idle" });
  const [activatingVersionID, setActivatingVersionID] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const selectedProject = useMemo(
    () => projects.find((project) => project.id === selectedProjectID) ?? null,
    [projects, selectedProjectID]
  );
  const activeVersion = useMemo(
    () => versions.find((version) => version.id === selectedProject?.activeVersionId) ?? null,
    [selectedProject?.activeVersionId, versions]
  );

  const reloadProjects = useCallback(async (preferredProjectID?: string) => {
    const nextProjects = await listProjects();
    setProjects(nextProjects);
    setSelectedProjectID((currentProjectID) => {
      if (preferredProjectID && nextProjects.some((project) => project.id === preferredProjectID)) {
        return preferredProjectID;
      }
      if (currentProjectID && nextProjects.some((project) => project.id === currentProjectID)) {
        return currentProjectID;
      }
      return nextProjects[0]?.id ?? null;
    });
  }, []);

  const refreshProjectData = useCallback(async (projectID: string) => {
    const [nextMessages, nextVersions] = await Promise.all([listMessages(projectID), listVersions(projectID)]);
    setSelectedProjectID((currentProjectID) => {
      if (currentProjectID === projectID) {
        setMessages(nextMessages);
        setVersions(nextVersions);
        setProjectDataState("ready");
      }
      return currentProjectID;
    });
    return { nextMessages, nextVersions };
  }, []);

  useEffect(() => {
    let active = true;
    void getHealth()
      .then((value) => {
        if (active) {
          setHealth({ kind: "ready", value });
        }
      })
      .catch(() => {
        if (active) {
          setHealth({ kind: "error" });
        }
      });
    return () => {
      active = false;
    };
  }, []);

  useEffect(() => {
    let active = true;
    void getSession()
      .then(async (session) => {
        if (!active) {
          return;
        }
        if (!session.initialized || !session.workspace) {
          setBootstrap("uninitialized");
          return;
        }
        setWorkspace(session.workspace);
        await reloadProjects();
        if (active) {
          setBootstrap("ready");
        }
      })
      .catch(() => {
        if (active) {
          setBootstrap("error");
        }
      });
    return () => {
      active = false;
    };
  }, [reloadProjects]);

  useEffect(() => {
    let active = true;
    if (!selectedProjectID) {
      setMessages([]);
      setVersions([]);
      setProjectDataState("idle");
      return () => {
        active = false;
      };
    }
    setProjectDataState("loading");
    void Promise.all([listMessages(selectedProjectID), listVersions(selectedProjectID)])
      .then(([nextMessages, nextVersions]) => {
        if (active) {
          setMessages(nextMessages);
          setVersions(nextVersions);
          setProjectDataState("ready");
        }
      })
      .catch(() => {
        if (active) {
          setProjectDataState("error");
        }
      });
    return () => {
      active = false;
    };
  }, [selectedProjectID]);

  async function handleInitialize(displayName: string) {
    setNotice(null);
    const session = await initializeSession(displayName);
    if (!session.workspace) {
      throw new Error("workspace was not returned");
    }
    setWorkspace(session.workspace);
    await reloadProjects();
    setBootstrap("ready");
  }

  async function handleCreateProject(name: string) {
    setNotice(null);
    const project = await createProject(name);
    await reloadProjects(project.id);
  }

  async function handleRenameProject(project: Project) {
    const name = window.prompt("项目名称", project.name);
    if (name === null || name.trim() === "" || name.trim() === project.name) {
      return;
    }
    try {
      const renamed = await renameProject(project.id, name);
      await reloadProjects(renamed.id);
      setNotice("项目名称已更新。");
    } catch (error) {
      setNotice(toMessage(error));
    }
  }

  async function handleDeleteProject(project: Project) {
    if (!window.confirm(`确定删除“${project.name}”吗？此操作不可恢复。`)) {
      return;
    }
    try {
      await deleteProject(project.id);
      await reloadProjects();
      setNotice("项目已删除。");
    } catch (error) {
      setNotice(toMessage(error));
    }
  }

  async function handleGenerate(project: Project, userRequest: string) {
    setNotice(null);
    setGeneration({ kind: "running", projectID: project.id, status: "requesting_model", label: "正在请求模型…" });
    let receivedResult = false;
    try {
      await generateProject(project.id, userRequest, (event: GenerationEvent) => {
        if (event.type === "stage") {
          setGeneration({ kind: "running", projectID: project.id, status: event.status, label: event.label });
          return;
        }
        if (event.type === "result") {
          receivedResult = true;
          return;
        }
        setGeneration({ kind: "failed", projectID: project.id, message: event.message, retryable: event.retryable });
      });
      if (receivedResult) {
        await Promise.all([reloadProjects(), refreshProjectData(project.id)]);
        setNotice("已生成新的可运行应用。可在右侧预览或查看编译代码。");
      }
      setGeneration({ kind: "idle" });
    } catch (error) {
      const message = toMessage(error);
      const retryable = error instanceof APIRequestError ? error.retryable : true;
      setGeneration({ kind: "failed", projectID: project.id, message, retryable });
      await refreshProjectData(project.id).catch(() => undefined);
      throw error;
    }
  }

  async function handleActivateVersion(project: Project, version: GenerationVersion) {
    if (project.activeVersionId === version.id) {
      return;
    }
    setActivatingVersionID(version.id);
    setNotice(null);
    try {
      await activateVersion(project.id, version.id);
      await Promise.all([reloadProjects(project.id), refreshProjectData(project.id)]);
      setNotice(`已恢复到版本 v${version.sequence}。`);
    } catch (error) {
      setNotice(toMessage(error));
    } finally {
      setActivatingVersionID(null);
    }
  }

  if (bootstrap === "loading") {
    return <LoadingScreen />;
  }
  if (bootstrap === "error") {
    return <ErrorScreen />;
  }
  if (bootstrap === "uninitialized") {
    return <WelcomeScreen health={health} onInitialize={handleInitialize} />;
  }

  return (
    <main className="min-h-screen bg-[#08080c] text-zinc-100">
      <div className="mx-auto flex min-h-screen max-w-[1680px] flex-col px-4 py-4 sm:px-5 lg:px-8 lg:py-5">
        <header className="flex items-center justify-between gap-4 border-b border-white/10 pb-4 sm:pb-5">
          <Brand workspace={workspace} />
          <HealthBadge health={health} />
        </header>
        {notice ? <p className="mt-4 rounded-xl border border-emerald-300/15 bg-emerald-300/[0.07] px-3 py-2 text-sm text-emerald-100">{notice}</p> : null}

        <section className="grid flex-1 gap-4 py-4 lg:grid-cols-[248px_minmax(0,1fr)_minmax(330px,420px)] lg:py-5">
          <ProjectSidebar
            projects={projects}
            selectedProjectID={selectedProjectID}
            onCreate={handleCreateProject}
            onSelect={setSelectedProjectID}
          />
          <WorkspacePanel
            generation={generation}
            health={health}
            messages={messages}
            onDelete={handleDeleteProject}
            onGenerate={handleGenerate}
            onRename={handleRenameProject}
            project={selectedProject}
            projectDataState={projectDataState}
            version={activeVersion}
          />
          <PreviewPanel
            activeVersion={activeVersion}
            activatingVersionID={activatingVersionID}
            onActivateVersion={handleActivateVersion}
            project={selectedProject}
            versions={versions}
          />
        </section>
      </div>
    </main>
  );
}

function WelcomeScreen({ health, onInitialize }: { health: HealthState; onInitialize: (displayName: string) => Promise<void> }) {
  const [displayName, setDisplayName] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      await onInitialize(displayName);
    } catch (reason) {
      setError(toMessage(reason));
      setSubmitting(false);
    }
  }

  return (
    <main className="grid min-h-screen place-items-center bg-[#08080c] px-5 text-zinc-100">
      <section className="w-full max-w-md rounded-3xl border border-white/10 bg-white/[0.03] p-7 shadow-2xl shadow-black/30">
        <div className="grid size-11 place-items-center rounded-2xl bg-violet-500 text-lg font-semibold shadow-lg shadow-violet-500/20">A</div>
        <p className="mt-6 text-xs font-medium uppercase tracking-[0.18em] text-violet-300">Welcome to Atoms Demo</p>
        <h1 className="mt-2 text-3xl font-semibold tracking-tight">开始你的本地工作区</h1>
        <p className="mt-3 text-sm leading-6 text-zinc-400">给工作区起个名字。项目、对话和生成版本会保存在本机 SQLite 数据库中。</p>
        <form className="mt-7" onSubmit={submit}>
          <label className="text-sm font-medium text-zinc-200" htmlFor="display-name">你的昵称</label>
          <input
            className="mt-2 w-full rounded-xl border border-white/10 bg-black/20 px-3 py-3 text-sm outline-none placeholder:text-zinc-600 focus:border-violet-400"
            id="display-name"
            maxLength={80}
            onChange={(event) => setDisplayName(event.target.value)}
            placeholder="例如 Zand"
            required
            value={displayName}
          />
          {error ? <p className="mt-3 text-sm text-rose-300">{error}</p> : null}
          <button className="mt-5 w-full rounded-xl bg-white px-4 py-3 text-sm font-semibold text-zinc-900 transition hover:bg-zinc-200 disabled:opacity-60" disabled={submitting} type="submit">
            {submitting ? "正在创建…" : "创建本地工作区"}
          </button>
        </form>
        <div className="mt-6 border-t border-white/10 pt-4"><HealthBadge health={health} /></div>
      </section>
    </main>
  );
}

function Brand({ workspace }: { workspace: Workspace | null }) {
  return (
    <div className="flex items-center gap-3">
      <div className="grid size-9 place-items-center rounded-xl bg-violet-500 font-semibold shadow-lg shadow-violet-500/20">A</div>
      <div>
        <p className="text-sm font-semibold tracking-tight">Atoms Demo</p>
        <p className="text-xs text-zinc-500">{workspace?.displayName ?? "Local workspace"} · Go backend</p>
      </div>
    </div>
  );
}

function ProjectSidebar({
  projects,
  selectedProjectID,
  onCreate,
  onSelect
}: {
  projects: Project[];
  selectedProjectID: string | null;
  onCreate: (name: string) => Promise<void>;
  onSelect: (projectID: string) => void;
}) {
  const [name, setName] = useState("");
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setCreating(true);
    setError(null);
    try {
      await onCreate(name);
      setName("");
    } catch (reason) {
      setError(toMessage(reason));
    } finally {
      setCreating(false);
    }
  }

  return (
    <aside className="rounded-2xl border border-white/10 bg-white/[0.03] p-4">
      <p className="text-xs font-medium uppercase tracking-[0.18em] text-zinc-500">Projects</p>
      <form className="mt-4 space-y-2" onSubmit={submit}>
        <input
          aria-label="新项目名称"
          className="w-full rounded-xl border border-white/10 bg-black/20 px-3 py-2.5 text-sm outline-none placeholder:text-zinc-600 focus:border-violet-400"
          maxLength={100}
          onChange={(event) => setName(event.target.value)}
          placeholder="新项目名称"
          required
          value={name}
        />
        <button className="w-full rounded-xl bg-white px-3 py-2.5 text-sm font-medium text-zinc-900 transition hover:bg-zinc-200 disabled:opacity-60" disabled={creating} type="submit">
          {creating ? "正在创建…" : "+ New project"}
        </button>
      </form>
      {error ? <p className="mt-3 text-xs leading-5 text-rose-300">{error}</p> : null}
      <div className="mt-6 space-y-1">
        {projects.length === 0 ? (
          <div className="rounded-xl border border-dashed border-white/10 px-3 py-8 text-center text-sm text-zinc-500">创建第一个项目，开始描述你想构建的应用。</div>
        ) : (
          projects.map((project) => (
            <button
              className={`w-full rounded-xl px-3 py-3 text-left text-sm transition ${project.id === selectedProjectID ? "bg-violet-500/15 text-violet-100" : "text-zinc-400 hover:bg-white/[0.05] hover:text-zinc-200"}`}
              key={project.id}
              onClick={() => onSelect(project.id)}
              type="button"
            >
              <span className="block truncate font-medium">{project.name}</span>
              <span className="mt-1 block text-xs text-zinc-500">{project.summary || formatDate(project.updatedAt)}</span>
            </button>
          ))
        )}
      </div>
    </aside>
  );
}

function WorkspacePanel({
  project,
  messages,
  version,
  generation,
  health,
  projectDataState,
  onRename,
  onDelete,
  onGenerate
}: {
  project: Project | null;
  messages: Message[];
  version: GenerationVersion | null;
  generation: GenerationState;
  health: HealthState;
  projectDataState: ProjectDataState;
  onRename: (project: Project) => Promise<void>;
  onDelete: (project: Project) => Promise<void>;
  onGenerate: (project: Project, request: string) => Promise<void>;
}) {
  if (!project) {
    return (
      <section className="grid min-h-[460px] place-items-center rounded-2xl border border-white/10 bg-white/[0.03] p-6 text-center">
        <div>
          <p className="text-xs font-medium uppercase tracking-[0.18em] text-violet-300">Agent workspace</p>
          <h1 className="mt-3 text-2xl font-semibold tracking-tight">先创建或选择一个项目</h1>
          <p className="mt-3 max-w-md text-sm leading-6 text-zinc-400">项目会自动保存到本地。创建后用自然语言描述待办、笔记或习惯打卡应用，Agent 会产生受控规格和可运行预览。</p>
        </div>
      </section>
    );
  }

  const isGenerating = generation.kind === "running" && generation.projectID === project.id;
  const generationFailure = generation.kind === "failed" && generation.projectID === project.id ? generation : null;
  const modelReady = health.kind === "ready" && health.value.model.configured;

  return (
    <section className="flex min-h-[600px] flex-col rounded-2xl border border-white/10 bg-white/[0.03] p-5">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <p className="text-xs font-medium uppercase tracking-[0.18em] text-violet-300">Agent workspace</p>
          <h1 className="mt-2 text-2xl font-semibold tracking-tight">{project.name}</h1>
          <p className="mt-2 max-w-2xl text-sm leading-6 text-zinc-400">{project.summary || "描述一个待办、笔记或习惯打卡应用，模型会返回受控规格，再由 Go 编译成独立的 HTML、CSS 和 JavaScript。"}</p>
        </div>
        <div className="flex gap-2">
          <button className="rounded-lg border border-white/10 px-3 py-2 text-sm text-zinc-300 hover:bg-white/[0.05]" onClick={() => void onRename(project)} type="button">Rename</button>
          <button className="rounded-lg border border-rose-400/20 px-3 py-2 text-sm text-rose-200 hover:bg-rose-400/10" onClick={() => void onDelete(project)} type="button">Delete</button>
        </div>
      </div>

      <GenerationStatus failure={generationFailure} running={isGenerating ? generation : null} />
      {projectDataState === "loading" ? <p className="mt-6 text-sm text-zinc-500">正在恢复项目记录…</p> : null}
      {projectDataState === "error" ? <p className="mt-6 rounded-xl border border-rose-400/20 bg-rose-400/10 px-3 py-2 text-sm text-rose-100">无法读取项目历史，请刷新后重试。</p> : null}
      {version ? <PlanCard version={version} /> : null}
      <MessageTimeline messages={messages} />
      <PromptComposer disabled={!modelReady || isGenerating} failure={generationFailure} onGenerate={(request) => onGenerate(project, request)} />
      {!modelReady ? <p className="mt-3 text-xs leading-5 text-amber-100/80">模型配置尚未完成。补充缺失的环境变量并重启服务后，即可开始真实生成。</p> : null}
    </section>
  );
}

function GenerationStatus({ running, failure }: { running: Extract<GenerationState, { kind: "running" }> | null; failure: Extract<GenerationState, { kind: "failed" }> | null }) {
  if (running) {
    return <div className="mt-5 flex items-center gap-3 rounded-xl border border-violet-400/20 bg-violet-400/10 px-3 py-2.5 text-sm text-violet-100"><span className="size-2 animate-pulse rounded-full bg-violet-300" />{running.label}</div>;
  }
  if (failure) {
    return <div className="mt-5 rounded-xl border border-rose-400/20 bg-rose-400/10 px-3 py-2.5 text-sm text-rose-100">{failure.message}{failure.retryable ? " 你可以修改或直接重试这条需求。" : ""}</div>;
  }
  return null;
}

function PlanCard({ version }: { version: GenerationVersion }) {
  return (
    <section className="mt-5 rounded-2xl border border-violet-400/15 bg-violet-400/[0.06] p-4">
      <div className="flex items-center justify-between gap-3">
        <p className="text-xs font-medium uppercase tracking-[0.16em] text-violet-200">Agent plan · v{version.sequence}</p>
        <span className="rounded-full bg-violet-300/10 px-2 py-1 text-xs text-violet-100">{version.spec.template}</span>
      </div>
      <p className="mt-2 text-sm font-medium leading-6 text-zinc-100">{version.plan.summary}</p>
      <ol className="mt-3 space-y-1.5 pl-4 text-sm leading-6 text-zinc-400">
        {version.plan.steps.map((step) => <li key={step}>{step}</li>)}
      </ol>
    </section>
  );
}

function MessageTimeline({ messages }: { messages: Message[] }) {
  if (messages.length === 0) {
    return <div className="mt-6 flex flex-1 items-center justify-center rounded-2xl border border-dashed border-white/10 bg-black/10 p-6 text-center text-sm leading-6 text-zinc-500">输入一段需求，例如“做一个深色主题的待办清单，包含工作和生活分类”。也可以创建笔记板或习惯打卡器；生成过程和 Agent 回复会保存在这里。</div>;
  }
  return (
    <div className="mt-6 flex min-h-0 flex-1 flex-col gap-3 overflow-auto pr-1">
      {messages.map((message) => (
        <article className={`rounded-2xl border px-3.5 py-3 ${messageTone(message.role)}`} key={message.id}>
          <div className="flex items-center justify-between gap-3 text-xs">
            <span className="font-medium uppercase tracking-[0.13em]">{messageLabel(message.role)}</span>
            <time className="text-zinc-500">{formatDate(message.createdAt)}</time>
          </div>
          <p className="mt-2 whitespace-pre-wrap text-sm leading-6 text-zinc-200">{message.content}</p>
        </article>
      ))}
    </div>
  );
}

function PromptComposer({ disabled, failure, onGenerate }: { disabled: boolean; failure: Extract<GenerationState, { kind: "failed" }> | null; onGenerate: (request: string) => Promise<void> }) {
  const [request, setRequest] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const normalized = request.trim();
    if (!normalized || disabled) {
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      await onGenerate(normalized);
      setRequest("");
    } catch (reason) {
      setError(toMessage(reason));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form className="mt-5 rounded-2xl border border-white/10 bg-black/20 p-3" onSubmit={submit}>
      <label className="sr-only" htmlFor="agent-request">应用需求</label>
      <textarea
        className="min-h-24 w-full resize-y rounded-xl border border-white/10 bg-white/[0.035] px-3 py-3 text-sm leading-6 outline-none placeholder:text-zinc-600 focus:border-violet-400 disabled:opacity-50"
        disabled={disabled || submitting}
        id="agent-request"
        maxLength={2000}
        onChange={(event) => setRequest(event.target.value)}
        placeholder="描述你想要的应用，例如：做一个海洋蓝主题的旅行准备清单，或一个灵感笔记板。"
        required
        value={request}
      />
      <div className="mt-3 flex items-center justify-between gap-3">
        <span className="text-xs text-zinc-500">{request.length}/2,000 · 仅生成受控小型单页应用</span>
        <button className="rounded-lg bg-violet-500 px-4 py-2 text-sm font-medium text-white transition hover:bg-violet-400 disabled:opacity-50" disabled={disabled || submitting || request.trim() === ""} type="submit">
          {submitting ? "正在生成…" : failure?.retryable ? "Retry generation" : "Generate"}
        </button>
      </div>
      {error ? <p className="mt-3 text-sm text-rose-300">{error}</p> : null}
    </form>
  );
}

function PreviewPanel({
  project,
  activeVersion,
  versions,
  activatingVersionID,
  onActivateVersion
}: {
  project: Project | null;
  activeVersion: GenerationVersion | null;
  versions: GenerationVersion[];
  activatingVersionID: string | null;
  onActivateVersion: (project: Project, version: GenerationVersion) => Promise<void>;
}) {
  const [tab, setTab] = useState<PreviewTab>("preview");
  const [codeView, setCodeView] = useState<"html" | "css" | "js">("html");

  if (!project) {
    return <PreviewEmpty title="Select a project" description="选择项目后，这里会显示隔离预览、受控规格和编译代码。" />;
  }
  if (!activeVersion) {
    return <PreviewEmpty title="No app generated yet" description="提交需求后，右侧会加载真实可交互的 sandbox 预览。" />;
  }

  const source = codeView === "html" ? activeVersion.artifact.html : codeView === "css" ? activeVersion.artifact.css : activeVersion.artifact.js;
  return (
    <aside className="flex min-h-[600px] flex-col overflow-hidden rounded-2xl border border-white/10 bg-white/[0.03]">
      <div className="flex items-center justify-between border-b border-white/10 px-4 py-3">
        <div>
          <p className="text-xs font-medium uppercase tracking-[0.18em] text-zinc-500">App preview</p>
          <p className="mt-1 text-xs text-zinc-500">v{activeVersion.sequence} · {activeVersion.spec.title}</p>
        </div>
        <span className="rounded-full border border-emerald-400/20 bg-emerald-400/10 px-2 py-1 text-xs text-emerald-100">Sandboxed</span>
      </div>
      <div className="flex gap-1 border-b border-white/10 px-3 pt-2">
        {(["preview", "code", "spec"] as PreviewTab[]).map((item) => (
          <button className={`rounded-t-lg px-3 py-2 text-xs font-medium ${tab === item ? "bg-white/[0.08] text-zinc-100" : "text-zinc-500 hover:text-zinc-300"}`} key={item} onClick={() => setTab(item)} type="button">{item === "preview" ? "预览" : item === "code" ? "代码" : "规格"}</button>
        ))}
      </div>
      {tab === "preview" ? <SandboxPreview key={activeVersion.id} projectID={project.id} projectName={project.name} version={activeVersion} /> : null}
      {tab === "code" ? (
        <div className="flex min-h-0 flex-1 flex-col">
          <div className="flex gap-1 border-b border-white/10 px-3 py-2">
            {(["html", "css", "js"] as const).map((item) => <button className={`rounded-md px-2 py-1 text-xs ${codeView === item ? "bg-violet-400/15 text-violet-100" : "text-zinc-500 hover:text-zinc-300"}`} key={item} onClick={() => setCodeView(item)} type="button">{item.toUpperCase()}</button>)}
          </div>
          <pre className="min-h-0 flex-1 overflow-auto whitespace-pre-wrap break-words p-4 text-xs leading-5 text-zinc-300"><code>{source}</code></pre>
        </div>
      ) : null}
      {tab === "spec" ? <pre className="min-h-0 flex-1 overflow-auto whitespace-pre-wrap break-words p-4 text-xs leading-5 text-zinc-300"><code>{JSON.stringify(activeVersion.spec, null, 2)}</code></pre> : null}
      <VersionList activeVersion={activeVersion} activatingVersionID={activatingVersionID} onActivate={(version) => onActivateVersion(project, version)} versions={versions} />
    </aside>
  );
}

function SandboxPreview({ projectID, projectName, version }: { projectID: string; projectName: string; version: GenerationVersion }) {
  const frameRef = useRef<HTMLIFrameElement>(null);
  const stateRef = useRef<unknown>(null);
  const saveTimerRef = useRef<number | null>(null);
  const pendingStateRef = useRef<unknown>(null);
  const [stateLoading, setStateLoading] = useState(true);
  const [stateError, setStateError] = useState<string | null>(null);

  const postInit = useCallback(() => {
    frameRef.current?.contentWindow?.postMessage({ type: "atoms-preview:init", versionId: version.id, state: stateRef.current }, "*");
  }, [version.id]);

  useEffect(() => {
    let active = true;
    setStateLoading(true);
    setStateError(null);
    void getPreviewState(projectID, version.id)
      .then((state) => {
        if (!active) {
          return;
        }
        stateRef.current = state;
        setStateLoading(false);
        postInit();
      })
      .catch((error) => {
        if (active) {
          setStateLoading(false);
          setStateError(toMessage(error));
        }
      });
    return () => {
      active = false;
    };
  }, [postInit, projectID, version.id]);

  useEffect(() => {
    function queueStateSave(state: unknown) {
      pendingStateRef.current = state;
      if (saveTimerRef.current !== null) {
        window.clearTimeout(saveTimerRef.current);
      }
      saveTimerRef.current = window.setTimeout(() => {
        const pendingState = pendingStateRef.current;
        pendingStateRef.current = null;
        if (pendingState) {
          void savePreviewState(projectID, version.id, pendingState).catch((error) => setStateError(toMessage(error)));
        }
      }, 350);
    }

    function handleMessage(event: MessageEvent<unknown>) {
      if (event.source !== frameRef.current?.contentWindow || !event.data || typeof event.data !== "object") {
        return;
      }
      const data = event.data as { type?: unknown; versionId?: unknown; state?: unknown };
      if (data.versionId !== version.id) {
        return;
      }
      if (data.type === "atoms-preview:ready") {
        return;
      }
      if (data.type === "atoms-preview:state-change" && data.state && typeof data.state === "object") {
        queueStateSave(data.state);
      }
    }

    window.addEventListener("message", handleMessage);
    return () => {
      window.removeEventListener("message", handleMessage);
      if (saveTimerRef.current !== null) {
        window.clearTimeout(saveTimerRef.current);
        saveTimerRef.current = null;
      }
      const pendingState = pendingStateRef.current;
      pendingStateRef.current = null;
      if (pendingState) {
        void savePreviewState(projectID, version.id, pendingState).catch(() => undefined);
      }
    };
  }, [postInit, projectID, version.id]);

  return (
    <div className="relative min-h-[430px] flex-1">
      <iframe
        className="min-h-[430px] w-full border-0 bg-white"
        onLoad={postInit}
        ref={frameRef}
        sandbox="allow-scripts"
        srcDoc={version.artifact.entryHtml}
        title={`${projectName} 预览`}
      />
      {stateLoading ? <span className="absolute right-3 top-3 rounded-full bg-zinc-950/75 px-2 py-1 text-xs text-zinc-300">正在恢复预览状态…</span> : null}
      {stateError ? <p className="absolute bottom-3 left-3 right-3 rounded-lg border border-rose-300/30 bg-rose-950/80 px-2 py-1.5 text-xs text-rose-100">预览状态未保存：{stateError}</p> : null}
    </div>
  );
}

function VersionList({
  activeVersion,
  versions,
  activatingVersionID,
  onActivate
}: {
  activeVersion: GenerationVersion;
  versions: GenerationVersion[];
  activatingVersionID: string | null;
  onActivate: (version: GenerationVersion) => Promise<void>;
}) {
  return (
    <section className="border-t border-white/10 px-4 py-3">
      <div className="flex items-center justify-between">
        <p className="text-xs font-medium uppercase tracking-[0.15em] text-zinc-500">Generated versions</p>
        <span className="text-xs text-zinc-600">{versions.length}</span>
      </div>
      <div className="mt-2 flex gap-2 overflow-auto pb-1">
        {versions.map((version) => (
          <button
            className={`shrink-0 rounded-lg border px-2 py-1.5 text-left text-xs transition disabled:cursor-default ${version.id === activeVersion.id ? "border-violet-400/30 bg-violet-400/10 text-violet-100" : "border-white/10 text-zinc-400 hover:border-white/20 hover:bg-white/[0.05]"}`}
            disabled={version.id === activeVersion.id || activatingVersionID !== null}
            key={version.id}
            onClick={() => void onActivate(version)}
            type="button"
          >
            <span className="block">v{version.sequence} · {formatDate(version.createdAt)}</span>
            <span className="mt-1 block text-[10px] opacity-75">{version.id === activeVersion.id ? "当前版本" : activatingVersionID === version.id ? "恢复中…" : "恢复此版本"}</span>
          </button>
        ))}
      </div>
      {versions.length > 1 ? <p className="mt-2 text-xs leading-5 text-zinc-600">选择历史版本即可恢复；所有生成产物都会保留在本地。</p> : null}
    </section>
  );
}

function PreviewEmpty({ title, description }: { title: string; description: string }) {
  return (
    <aside className="rounded-2xl border border-white/10 bg-white/[0.03] p-4">
      <p className="text-xs font-medium uppercase tracking-[0.18em] text-zinc-500">App preview</p>
      <div className="mt-4 grid min-h-[360px] place-items-center rounded-xl border border-dashed border-white/10 bg-black/10 p-6 text-center">
        <div>
          <p className="text-sm font-medium text-zinc-300">{title}</p>
          <p className="mt-2 text-sm leading-6 text-zinc-500">{description}</p>
        </div>
      </div>
    </aside>
  );
}

function LoadingScreen() {
  return <main className="grid min-h-screen place-items-center bg-[#08080c] text-sm text-zinc-500">正在恢复本地工作区…</main>;
}

function ErrorScreen() {
  return <main className="grid min-h-screen place-items-center bg-[#08080c] p-5 text-center text-zinc-200"><div><h1 className="text-xl font-semibold">无法连接本地服务</h1><p className="mt-2 text-sm text-zinc-500">请确认 Go 服务正在运行，然后刷新页面。</p></div></main>;
}

function HealthBadge({ health }: { health: HealthState }) {
  if (health.kind === "loading") {
    return <span className="rounded-full border border-white/10 px-3 py-1.5 text-xs text-zinc-500">Checking server…</span>;
  }
  if (health.kind === "error") {
    return <span className="rounded-full border border-rose-400/30 bg-rose-400/10 px-3 py-1.5 text-xs text-rose-200">Server unavailable</span>;
  }
  if (health.value.model.configured) {
    return <span className="rounded-full border border-emerald-400/30 bg-emerald-400/10 px-3 py-1.5 text-xs text-emerald-200">Model configured</span>;
  }
  return <span className="max-w-[52vw] truncate rounded-full border border-amber-400/30 bg-amber-400/10 px-3 py-1.5 text-xs text-amber-100">Missing: {health.value.model.missing.join(", ")}</span>;
}

function messageLabel(role: Message["role"]) {
  return role === "user" ? "You" : role === "assistant" ? "Agent" : "System";
}

function messageTone(role: Message["role"]) {
  if (role === "user") return "border-white/10 bg-white/[0.045]";
  if (role === "assistant") return "border-violet-400/15 bg-violet-400/[0.06]";
  return "border-amber-300/15 bg-amber-300/[0.05]";
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat("zh-CN", { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" }).format(new Date(value));
}

function toMessage(error: unknown) {
  if (error instanceof APIRequestError) {
    return error.message;
  }
  return "操作失败，请稍后再试。";
}
