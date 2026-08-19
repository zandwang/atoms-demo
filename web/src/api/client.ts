export type Workspace = {
  id: string;
  displayName: string;
  createdAt: string;
  updatedAt: string;
};

export type Project = {
  id: string;
  name: string;
  summary: string;
  activeVersionId?: string;
  createdAt: string;
  updatedAt: string;
};

export type ModelStatus = {
  available: boolean;
  defaultProviderConfigured: boolean;
  defaultConfigured: boolean;
};

export type ModelConfig = {
  baseURL: string;
  model: string;
  apiKey: string;
};

export type Health = {
  status: string;
  model: ModelStatus;
};

export type Session = {
  initialized: boolean;
  workspace: Workspace | null;
};

export type MessageRole = "user" | "assistant" | "system";

export type Message = {
  id: string;
  projectId: string;
  role: MessageRole;
  content: string;
  generationId?: string;
  createdAt: string;
};

export type AgentPlan = {
  summary: string;
  steps: string[];
  selectedTemplate?: "todo" | "notes" | "habits" | "custom";
};

export type TodoPriority = "low" | "medium" | "high";

export type TodoSpec = {
  schemaVersion: 1;
  template: "todo";
  title: string;
  description: string;
  theme: "violet" | "ocean" | "forest" | "sunset" | "slate";
  categories: string[];
  initialItems: Array<{
    text: string;
    category: string;
    priority: TodoPriority;
  }>;
};

export type NotesSpec = {
  schemaVersion: 1;
  template: "notes";
  title: string;
  description: string;
  theme: "violet" | "ocean" | "forest" | "sunset" | "slate";
  tags: string[];
  initialNotes: Array<{
    title: string;
    content: string;
    tags: string[];
  }>;
};

export type HabitsSpec = {
  schemaVersion: 1;
  template: "habits";
  title: string;
  description: string;
  theme: "violet" | "ocean" | "forest" | "sunset" | "slate";
  initialHabits: Array<{
    name: string;
    icon: "check" | "heart" | "book" | "run" | "water";
    targetPerWeek: number;
  }>;
};

export type CustomSpec = {
  schemaVersion: 1;
  template: "custom";
  files: { indexHtml: string; stylesCss: string; appJs: string };
};

export type AppSpec = TodoSpec | NotesSpec | HabitsSpec | CustomSpec;

export type ArtifactManifest = {
  template: "todo" | "notes" | "habits" | "custom";
  actions: string[];
  checksum: string;
};

export type CompiledArtifact = {
  entryHtml: string;
  html: string;
  css: string;
  js: string;
  manifest: ArtifactManifest;
};

export type GenerationVersion = {
  id: string;
  projectId: string;
  parentVersionId?: string;
  sequence: number;
  request: string;
  plan: AgentPlan;
  spec: AppSpec;
  artifact: CompiledArtifact;
  createdAt: string;
};

export type GenerationStage = {
  type: "stage";
  status: "preparing_context" | "requesting_model" | "validating" | "compiling" | "saving_version";
  label: string;
};

export type GenerationResult = {
  type: "result";
  version: GenerationVersion;
  message: string;
};

export type GenerationFailure = {
  type: "error";
  code: string;
  message: string;
  retryable: boolean;
};

export type GenerationEvent = GenerationStage | GenerationResult | GenerationFailure;

type APIErrorBody = {
  code: string;
  message: string;
  retryable: boolean;
};

type Envelope<T> = {
  data: T;
  error: APIErrorBody | null;
};

export class APIRequestError extends Error {
  readonly code: string;
  readonly retryable: boolean;

  constructor(error: APIErrorBody) {
    super(error.message);
    this.name = "APIRequestError";
    this.code = error.code;
    this.retryable = error.retryable;
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    ...init,
    headers: {
      ...(init?.body ? { "Content-Type": "application/json" } : {}),
      ...init?.headers
    }
  });

  const envelope = (await response.json()) as Envelope<T>;
  if (!response.ok || envelope.error) {
    throw new APIRequestError(
      envelope.error ?? {
        code: "HTTP_ERROR",
        message: "请求失败，请稍后再试。",
        retryable: response.status >= 500
      }
    );
  }

  return envelope.data;
}

export function getHealth() {
  return request<Health>("/api/health");
}

export function getSession() {
  return request<Session>("/api/session");
}

export function initializeSession(displayName: string) {
  return request<Session>("/api/session/initialize", {
    method: "POST",
    body: JSON.stringify({ displayName })
  });
}

export function listProjects() {
  return request<Project[]>("/api/projects");
}

export function createProject(name: string) {
  return request<Project>("/api/projects", {
    method: "POST",
    body: JSON.stringify({ name })
  });
}

export function renameProject(projectID: string, name: string) {
  return request<Project>(`/api/projects/${encodeURIComponent(projectID)}`, {
    method: "PATCH",
    body: JSON.stringify({ name })
  });
}

export function deleteProject(projectID: string) {
  return request<{ deleted: boolean }>(`/api/projects/${encodeURIComponent(projectID)}`, {
    method: "DELETE"
  });
}

export function listMessages(projectID: string) {
  return request<Message[]>(`/api/projects/${encodeURIComponent(projectID)}/messages`);
}

export function listVersions(projectID: string) {
  return request<GenerationVersion[]>(`/api/projects/${encodeURIComponent(projectID)}/versions`);
}

export function activateVersion(projectID: string, versionID: string) {
  return request<Project>(`/api/projects/${encodeURIComponent(projectID)}/versions/${encodeURIComponent(versionID)}/activate`, {
    method: "POST"
  });
}

export function getPreviewState(projectID: string, versionID: string) {
  return request<unknown>(`/api/projects/${encodeURIComponent(projectID)}/versions/${encodeURIComponent(versionID)}/preview-state`);
}

export function savePreviewState(projectID: string, versionID: string, state: unknown) {
  return request<{ saved: boolean }>(`/api/projects/${encodeURIComponent(projectID)}/versions/${encodeURIComponent(versionID)}/preview-state`, {
    method: "PUT",
    body: JSON.stringify({ state })
  });
}

export async function generateProject(
  projectID: string,
  userRequest: string,
  modelConfig: ModelConfig,
  onEvent: (event: GenerationEvent) => void
) {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  if (modelConfig.baseURL.trim()) {
    headers["X-Model-Base-URL"] = modelConfig.baseURL.trim();
  }
  if (modelConfig.model.trim()) {
    headers["X-Model-Name"] = modelConfig.model.trim();
  }
  if (modelConfig.apiKey.trim()) {
    headers["X-Model-API-Key"] = modelConfig.apiKey.trim();
  }
  const response = await fetch(`/api/projects/${encodeURIComponent(projectID)}/generate`, {
    method: "POST",
    headers,
    body: JSON.stringify({ userRequest })
  });

  if (!response.ok || !response.headers.get("Content-Type")?.includes("text/event-stream")) {
    const envelope = await response.json().catch(() => null) as Envelope<never> | null;
    throw new APIRequestError(
      envelope?.error ?? {
        code: "HTTP_ERROR",
        message: "无法开始生成，请稍后再试。",
        retryable: response.status >= 500
      }
    );
  }
  if (!response.body) {
    throw new APIRequestError({ code: "STREAM_ERROR", message: "浏览器无法读取生成进度。", retryable: true });
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  let streamError: APIRequestError | null = null;

  function dispatch(block: string) {
    const lines = block.replaceAll("\r", "").split("\n");
    const eventName = lines.find((line) => line.startsWith("event:"))?.slice("event:".length).trim();
    const payload = lines
      .filter((line) => line.startsWith("data:"))
      .map((line) => line.slice("data:".length).trimStart())
      .join("\n");
    if (!eventName || !payload) {
      return;
    }

    const data = JSON.parse(payload) as Omit<GenerationEvent, "type">;
    if (eventName === "stage") {
      onEvent({ type: "stage", ...(data as Omit<GenerationStage, "type">) });
      return;
    }
    if (eventName === "result") {
      onEvent({ type: "result", ...(data as Omit<GenerationResult, "type">) });
      return;
    }
    if (eventName === "error") {
      const failure = { type: "error", ...(data as Omit<GenerationFailure, "type">) } as GenerationFailure;
      onEvent(failure);
      streamError = new APIRequestError(failure);
    }
  }

  try {
    while (true) {
      const { done, value } = await reader.read();
      buffer += decoder.decode(value, { stream: !done });
      let boundary = buffer.indexOf("\n\n");
      while (boundary >= 0) {
        dispatch(buffer.slice(0, boundary));
        buffer = buffer.slice(boundary + 2);
        boundary = buffer.indexOf("\n\n");
      }
      if (done) {
        break;
      }
    }
    if (buffer.trim()) {
      dispatch(buffer);
    }
  } finally {
    reader.releaseLock();
  }

  if (streamError) {
    throw streamError;
  }
}
