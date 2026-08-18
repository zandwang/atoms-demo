package compiler

import "github.com/zand/atoms-demo/internal/domain"

// CompileNotes creates an editable, searchable note board from a validated spec.
func CompileNotes(spec domain.NotesSpec) (domain.CompiledArtifact, error) {
	if err := spec.NormalizeAndValidate(); err != nil {
		return domain.CompiledArtifact{}, err
	}
	return makeArtifact(domain.AppSpec{Notes: &spec}, notesHTML, notesCSS, notesJS, []string{"add", "edit", "delete", "search"})
}

const notesHTML = `<body>
  <main class="app-shell" aria-labelledby="app-title">
    <section class="app-card notes-card">
      <header class="app-header">
        <div>
          <p class="eyebrow">ATOMS · NOTES</p>
          <h1 id="app-title"></h1>
          <p id="app-description" class="description"></p>
        </div>
        <div id="note-count" class="progress" aria-live="polite"></div>
      </header>
      <form id="note-form" class="note-form">
        <input id="note-title" maxlength="100" autocomplete="off" placeholder="笔记标题" required>
        <select id="tag-select" aria-label="标签"></select>
        <textarea id="note-content" maxlength="800" placeholder="写下想记住的内容…"></textarea>
        <button id="note-submit" type="submit">保存笔记</button>
      </form>
      <div class="note-toolbar">
        <label class="sr-only" for="note-search">搜索笔记</label>
        <input id="note-search" maxlength="100" autocomplete="off" placeholder="搜索笔记…">
        <button id="cancel-edit" type="button" hidden>取消编辑</button>
      </div>
      <section id="note-list" class="note-list" aria-live="polite"></section>
      <p id="empty-state" class="empty-state" hidden>还没有匹配的笔记。记下一件值得保留的想法吧。</p>
    </section>
  </main>
</body>`

const notesCSS = todoCSS + `
.notes-card { max-width: 820px; }
.note-form { display: grid; grid-template-columns: minmax(0, 1fr) 136px auto; gap: 9px; padding: 20px 30px 10px; }
.note-form textarea { grid-column: 1 / -1; min-height: 90px; resize: vertical; border: 1px solid var(--border); border-radius: 11px; padding: 11px 12px; color: inherit; outline: none; }
.note-form textarea:focus { border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-soft); }
.note-form button { border: 0; border-radius: 11px; background: var(--accent); color: #fff; padding: 0 16px; font-weight: 750; }
.note-toolbar { display: flex; gap: 9px; padding: 4px 30px 18px; }
.note-toolbar input { flex: 1; }
.note-toolbar button { border: 1px solid var(--border); border-radius: 10px; background: transparent; color: #716a7d; padding: 0 12px; }
.note-list { display: grid; grid-template-columns: repeat(auto-fill, minmax(210px, 1fr)); gap: 10px; padding: 0 18px 18px; }
.note { display: flex; min-height: 165px; flex-direction: column; border: 1px solid var(--border); border-radius: 15px; background: #fff; padding: 15px; }
.note-title { margin: 0; overflow-wrap: anywhere; font-size: 15px; line-height: 1.35; }
.note-content { margin: 9px 0 0; flex: 1; white-space: pre-wrap; overflow-wrap: anywhere; color: #756e80; font-size: 13px; line-height: 1.55; }
.tag-row { display: flex; flex-wrap: wrap; gap: 5px; margin-top: 12px; }
.tag { border-radius: 999px; background: var(--accent-soft); color: var(--accent); padding: 4px 7px; font-size: 11px; font-weight: 700; }
.note-actions { display: flex; gap: 6px; margin-top: 13px; }
.note-actions button { border: 0; border-radius: 8px; background: #f3f0f7; color: #665f70; padding: 6px 8px; font-size: 12px; }
.note-actions .danger { color: #a04343; }
@media (max-width: 650px) {
  .note-form { grid-template-columns: 1fr; padding: 16px 20px 8px; }
  .note-form button { min-height: 42px; }
  .note-toolbar { padding: 4px 20px 16px; }
}`

const notesJS = `(() => {
  "use strict";
  const spec = JSON.parse(document.getElementById("atoms-spec").textContent);
  const preview = window.atomsPreview;
  const form = document.getElementById("note-form");
  const titleInput = document.getElementById("note-title");
  const contentInput = document.getElementById("note-content");
  const tagSelect = document.getElementById("tag-select");
  const searchInput = document.getElementById("note-search");
  const submitButton = document.getElementById("note-submit");
  const cancelButton = document.getElementById("cancel-edit");
  const noteList = document.getElementById("note-list");
  const emptyState = document.getElementById("empty-state");
  const count = document.getElementById("note-count");

  document.documentElement.dataset.theme = spec.theme;
  document.getElementById("app-title").textContent = spec.title;
  document.getElementById("app-description").textContent = spec.description;
  for (const tag of spec.tags) {
    const option = document.createElement("option");
    option.value = tag;
    option.textContent = tag;
    tagSelect.append(option);
  }

  let nextID = 1;
  let editingID = null;
  let query = "";
  const state = { notes: spec.initialNotes.map((note) => ({ ...note, id: String(nextID++) })) };

  function snapshot() {
    return { notes: state.notes.map((note) => ({ id: note.id, title: note.title, content: note.content, tags: [...note.tags] })) };
  }

  function restore(nextState) {
    if (!nextState || !Array.isArray(nextState.notes)) return;
    state.notes = nextState.notes
      .filter((note) => note && typeof note.id === "string" && typeof note.title === "string" && typeof note.content === "string" && Array.isArray(note.tags))
      .slice(0, 100)
      .map((note) => ({ id: note.id, title: note.title, content: note.content, tags: note.tags.filter((tag) => typeof tag === "string").slice(0, 3) }));
    nextID = state.notes.length + 1;
    editingID = null;
    render();
  }

  function resetEditor() {
    editingID = null;
    titleInput.value = "";
    contentInput.value = "";
    submitButton.textContent = "保存笔记";
    cancelButton.hidden = true;
  }

  function makeNote(note) {
    const card = document.createElement("article");
    card.className = "note";
    const heading = document.createElement("h2");
    heading.className = "note-title";
    heading.textContent = note.title;
    const content = document.createElement("p");
    content.className = "note-content";
    content.textContent = note.content || "（空白笔记）";
    const tags = document.createElement("div");
    tags.className = "tag-row";
    for (const tag of note.tags) {
      const chip = document.createElement("span");
      chip.className = "tag";
      chip.textContent = tag;
      tags.append(chip);
    }
    const actions = document.createElement("div");
    actions.className = "note-actions";
    const edit = document.createElement("button");
    edit.type = "button";
    edit.textContent = "编辑";
    edit.addEventListener("click", () => {
      editingID = note.id;
      titleInput.value = note.title;
      contentInput.value = note.content;
      tagSelect.value = note.tags[0] || spec.tags[0];
      submitButton.textContent = "更新笔记";
      cancelButton.hidden = false;
      titleInput.focus();
    });
    const remove = document.createElement("button");
    remove.className = "danger";
    remove.type = "button";
    remove.textContent = "删除";
    remove.addEventListener("click", () => {
      state.notes = state.notes.filter((item) => item !== note);
      if (editingID === note.id) resetEditor();
      render();
    });
    actions.append(edit, remove);
    card.append(heading, content, tags, actions);
    return card;
  }

  function render() {
    const normalizedQuery = query.toLocaleLowerCase();
    const visible = state.notes.filter((note) => (note.title + " " + note.content + " " + note.tags.join(" ")).toLocaleLowerCase().includes(normalizedQuery));
    count.textContent = state.notes.length + " 条笔记";
    noteList.replaceChildren(...visible.map(makeNote));
    emptyState.hidden = visible.length !== 0;
    preview.publish(snapshot());
  }

  form.addEventListener("submit", (event) => {
    event.preventDefault();
    const title = titleInput.value.trim();
    if (!title) return;
    const content = contentInput.value.trim();
    const tags = tagSelect.value ? [tagSelect.value] : [];
    if (editingID) {
      const note = state.notes.find((item) => item.id === editingID);
      if (note) Object.assign(note, { title, content, tags });
    } else {
      state.notes.unshift({ id: String(nextID++), title, content, tags });
    }
    resetEditor();
    render();
  });
  searchInput.addEventListener("input", () => { query = searchInput.value; render(); });
  cancelButton.addEventListener("click", () => resetEditor());
  preview.onRestore(restore);
  render();
})();`
