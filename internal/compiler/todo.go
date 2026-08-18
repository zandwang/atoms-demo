package compiler

import (
	"github.com/zand/atoms-demo/internal/domain"
)

// CompileTodo creates a single-file, interactive todo application. Dynamic
// text remains JSON data and is written through textContent at runtime.
func CompileTodo(spec domain.TodoSpec) (domain.CompiledArtifact, error) {
	if err := spec.NormalizeAndValidate(); err != nil {
		return domain.CompiledArtifact{}, err
	}

	return makeArtifact(domain.AppSpec{Todo: &spec}, todoHTML, todoCSS, todoJS, []string{"add", "toggle", "delete", "filter"})
}

const todoHTML = `<body>
  <main class="app-shell" aria-labelledby="app-title">
    <section class="app-card">
      <header class="app-header">
        <div>
          <p class="eyebrow">ATOMS · TODO</p>
          <h1 id="app-title"></h1>
          <p id="app-description" class="description"></p>
        </div>
        <div id="progress" class="progress" aria-live="polite"></div>
      </header>

      <form id="task-form" class="task-form">
        <label class="sr-only" for="task-input">添加待办</label>
        <input id="task-input" name="task" maxlength="160" autocomplete="off" placeholder="添加一件要做的事…" required>
        <label class="sr-only" for="category-select">分类</label>
        <select id="category-select" aria-label="分类"></select>
        <label class="sr-only" for="priority-select">优先级</label>
        <select id="priority-select" aria-label="优先级">
          <option value="low">低优先级</option>
          <option value="medium" selected>中优先级</option>
          <option value="high">高优先级</option>
        </select>
        <button type="submit">添加</button>
      </form>

      <div class="toolbar" role="group" aria-label="待办筛选">
        <button class="filter is-active" data-filter="all" type="button">全部</button>
        <button class="filter" data-filter="active" type="button">进行中</button>
        <button class="filter" data-filter="done" type="button">已完成</button>
      </div>

      <ul id="task-list" class="task-list" aria-live="polite"></ul>
      <p id="empty-state" class="empty-state" hidden>这里还没有待办。写下第一件想完成的小事吧。</p>
    </section>
  </main>
</body>`

const todoCSS = `:root {
  color-scheme: light;
  font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  background: #f5f3ff;
  color: #231f3a;
}

* { box-sizing: border-box; }
body { margin: 0; min-width: 280px; }
button, input, select { font: inherit; }
button { cursor: pointer; }
.app-shell { min-height: 100vh; padding: 24px; display: grid; place-items: start center; }
.app-card { width: min(100%, 720px); border: 1px solid var(--border); border-radius: 24px; background: rgba(255,255,255,.92); box-shadow: 0 24px 70px rgba(44, 26, 82, .13); overflow: hidden; }
.app-header { display: flex; justify-content: space-between; gap: 20px; padding: 30px 30px 24px; border-bottom: 1px solid var(--border); }
.eyebrow { margin: 0; color: var(--accent); font-size: 11px; font-weight: 800; letter-spacing: .14em; }
h1 { margin: 8px 0 0; font-size: clamp(27px, 6vw, 42px); line-height: 1.05; letter-spacing: -.045em; }
.description { max-width: 520px; margin: 10px 0 0; color: #6c6680; line-height: 1.55; font-size: 14px; }
.progress { align-self: start; white-space: nowrap; border-radius: 999px; background: var(--accent-soft); color: var(--accent); padding: 8px 11px; font-size: 12px; font-weight: 750; }
.task-form { display: grid; grid-template-columns: minmax(0, 1fr) 124px 112px auto; gap: 9px; padding: 20px 30px 14px; }
input, select { min-width: 0; border: 1px solid var(--border); border-radius: 11px; background: #fff; color: inherit; outline: none; padding: 11px 12px; }
input:focus, select:focus { border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-soft); }
.task-form > button { border: 0; border-radius: 11px; background: var(--accent); color: #fff; padding: 0 16px; font-weight: 750; }
.toolbar { display: flex; gap: 7px; padding: 0 30px 16px; }
.filter { border: 1px solid var(--border); border-radius: 999px; background: transparent; color: #736d84; padding: 7px 11px; font-size: 12px; }
.filter.is-active { border-color: transparent; background: var(--accent-soft); color: var(--accent); font-weight: 750; }
.task-list { list-style: none; margin: 0; padding: 0 18px 18px; }
.task { display: flex; align-items: center; gap: 11px; border-top: 1px solid var(--border); padding: 14px 12px; }
.toggle { appearance: none; width: 19px; height: 19px; flex: 0 0 auto; margin: 0; border: 2px solid var(--accent); border-radius: 6px; display: grid; place-content: center; }
.toggle::before { content: ""; width: 9px; height: 5px; border: solid #fff; border-width: 0 0 2px 2px; transform: rotate(-45deg) scale(0); transition: transform .14s ease; }
.toggle:checked { background: var(--accent); }
.toggle:checked::before { transform: rotate(-45deg) scale(1); }
.task-copy { min-width: 0; flex: 1; }
.task-text { display: block; overflow-wrap: anywhere; font-size: 14px; font-weight: 650; }
.task.done .task-text { color: #918a9f; text-decoration: line-through; }
.task-meta { display: block; margin-top: 3px; color: #918a9f; font-size: 12px; }
.priority { flex: 0 0 auto; border-radius: 999px; padding: 5px 8px; background: #f2eff8; color: #746d84; font-size: 11px; font-weight: 700; }
.priority.high { background: #ffe9e5; color: #b14535; }
.priority.low { background: #e7f4ef; color: #257052; }
.delete { flex: 0 0 auto; border: 0; border-radius: 8px; background: transparent; color: #a29cab; padding: 7px 8px; }
.delete:hover, .delete:focus-visible { background: #f5f1f7; color: #5d566a; }
.empty-state { margin: 0 30px 28px; border: 1px dashed var(--border); border-radius: 14px; color: #8b8494; padding: 28px 20px; text-align: center; font-size: 14px; }
.sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0, 0, 0, 0); white-space: nowrap; border: 0; }
[data-theme="violet"] { --accent: #7356d9; --accent-soft: #eeeaff; --border: #e7e2f0; background: #f5f3ff; }
[data-theme="ocean"] { --accent: #2679b7; --accent-soft: #e4f3ff; --border: #dceaf2; background: #f1f8fc; }
[data-theme="forest"] { --accent: #24734d; --accent-soft: #e4f4ea; --border: #d9eadd; background: #f0f8f2; }
[data-theme="sunset"] { --accent: #b75a38; --accent-soft: #ffebe3; --border: #f0dfd7; background: #fff7f3; }
[data-theme="slate"] { --accent: #45586e; --accent-soft: #e9eef3; --border: #dce3ea; background: #f3f6f8; }
@media (max-width: 650px) {
  .app-shell { padding: 12px; }
  .app-header { padding: 24px 20px 20px; }
  .task-form { grid-template-columns: 1fr 1fr; padding: 16px 20px 12px; }
  .task-form input { grid-column: 1 / -1; }
  .task-form > button { min-height: 42px; }
  .toolbar { padding: 0 20px 13px; }
  .empty-state { margin: 0 20px 20px; }
}`

const todoJS = `(() => {
  "use strict";

  const specNode = document.getElementById("atoms-spec");
  const spec = JSON.parse(specNode.textContent);
  const root = document.documentElement;
  const taskForm = document.getElementById("task-form");
  const taskInput = document.getElementById("task-input");
  const categorySelect = document.getElementById("category-select");
  const prioritySelect = document.getElementById("priority-select");
  const taskList = document.getElementById("task-list");
  const emptyState = document.getElementById("empty-state");
  const progress = document.getElementById("progress");
  const preview = window.atomsPreview;

  root.dataset.theme = spec.theme;
  document.getElementById("app-title").textContent = spec.title;
  document.getElementById("app-description").textContent = spec.description;

  for (const category of spec.categories) {
    const option = document.createElement("option");
    option.value = category;
    option.textContent = category;
    categorySelect.append(option);
  }

  let nextID = 1;
  let filter = "all";
  const state = {
    items: spec.initialItems.map((item) => ({ ...item, id: String(nextID++), done: false }))
  };

  function priorityLabel(priority) {
    return { low: "低优先级", medium: "中优先级", high: "高优先级" }[priority] || "中优先级";
  }

  function snapshot() {
    return { items: state.items.map((item) => ({ id: item.id, text: item.text, category: item.category, priority: item.priority, done: Boolean(item.done) })) };
  }

  function restore(nextState) {
    if (!nextState || !Array.isArray(nextState.items)) return;
    state.items = nextState.items
      .filter((item) => item && typeof item.id === "string" && typeof item.text === "string" && typeof item.category === "string" && typeof item.priority === "string" && typeof item.done === "boolean")
      .slice(0, 100)
      .map((item) => ({ id: item.id, text: item.text, category: item.category, priority: item.priority, done: item.done }));
    nextID = state.items.length + 1;
    render();
  }

  function makeTask(item) {
    const row = document.createElement("li");
    row.className = "task" + (item.done ? " done" : "");

    const toggle = document.createElement("input");
    toggle.className = "toggle";
    toggle.type = "checkbox";
    toggle.checked = item.done;
    toggle.setAttribute("aria-label", "切换待办完成状态");
    toggle.addEventListener("change", () => {
      item.done = toggle.checked;
      render();
    });

    const copy = document.createElement("div");
    copy.className = "task-copy";
    const text = document.createElement("span");
    text.className = "task-text";
    text.textContent = item.text;
    const meta = document.createElement("span");
    meta.className = "task-meta";
    meta.textContent = item.category + " · " + priorityLabel(item.priority);
    copy.append(text, meta);

    const badge = document.createElement("span");
    badge.className = "priority " + item.priority;
    badge.textContent = priorityLabel(item.priority);

    const remove = document.createElement("button");
    remove.className = "delete";
    remove.type = "button";
    remove.textContent = "删除";
    remove.addEventListener("click", () => {
      const index = state.items.indexOf(item);
      if (index >= 0) {
        state.items.splice(index, 1);
        render();
      }
    });

    row.append(toggle, copy, badge, remove);
    return row;
  }

  function render() {
    const visible = state.items.filter((item) => filter === "all" || filter === "done" && item.done || filter === "active" && !item.done);
    const completed = state.items.filter((item) => item.done).length;
    progress.textContent = state.items.length === 0 ? "准备开始" : completed + "/" + state.items.length + " 已完成";
    taskList.replaceChildren(...visible.map(makeTask));
    emptyState.hidden = visible.length !== 0;
    for (const button of document.querySelectorAll(".filter")) {
      button.classList.toggle("is-active", button.dataset.filter === filter);
    }
    preview.publish(snapshot());
  }

  taskForm.addEventListener("submit", (event) => {
    event.preventDefault();
    const text = taskInput.value.trim();
    if (!text) return;
    state.items.unshift({ id: String(nextID++), text, category: categorySelect.value, priority: prioritySelect.value, done: false });
    taskInput.value = "";
    taskInput.focus();
    render();
  });

  for (const button of document.querySelectorAll(".filter")) {
    button.addEventListener("click", () => {
      filter = button.dataset.filter;
      render();
    });
  }

  preview.onRestore(restore);
  render();
})();`
