package compiler

import "github.com/zand/atoms-demo/internal/domain"

// CompileHabits creates a daily habit tracker with real check-in and streak logic.
func CompileHabits(spec domain.HabitsSpec) (domain.CompiledArtifact, error) {
	if err := spec.NormalizeAndValidate(); err != nil {
		return domain.CompiledArtifact{}, err
	}
	return makeArtifact(domain.AppSpec{Habits: &spec}, habitsHTML, habitsCSS, habitsJS, []string{"add", "check-in", "streak", "delete"})
}

const habitsHTML = `<body>
  <main class="app-shell" aria-labelledby="app-title">
    <section class="app-card habits-card">
      <header class="app-header">
        <div>
          <p class="eyebrow">ATOMS · HABITS</p>
          <h1 id="app-title"></h1>
          <p id="app-description" class="description"></p>
        </div>
        <div id="day-label" class="progress" aria-live="polite"></div>
      </header>
      <form id="habit-form" class="habit-form">
        <input id="habit-name" maxlength="80" autocomplete="off" placeholder="添加一个习惯" required>
        <select id="icon-select" aria-label="图标">
          <option value="check">✓ 专注</option>
          <option value="heart">♥ 身心</option>
          <option value="book">▤ 学习</option>
          <option value="run">↗ 运动</option>
          <option value="water">≈ 健康</option>
        </select>
        <select id="target-select" aria-label="每周目标">
          <option value="7">每天</option>
          <option value="5">每周 5 天</option>
          <option value="3">每周 3 天</option>
          <option value="1">每周 1 天</option>
        </select>
        <button type="submit">添加</button>
      </form>
      <section id="habit-list" class="habit-list" aria-live="polite"></section>
      <p id="empty-state" class="empty-state" hidden>还没有习惯。添加一个每天愿意为自己完成的小动作吧。</p>
    </section>
  </main>
</body>`

const habitsCSS = todoCSS + `
.habits-card { max-width: 800px; }
.habit-form { display: grid; grid-template-columns: minmax(0, 1fr) 120px 120px auto; gap: 9px; padding: 20px 30px 18px; }
.habit-form button { border: 0; border-radius: 11px; background: var(--accent); color: #fff; padding: 0 16px; font-weight: 750; }
.habit-list { display: grid; gap: 9px; padding: 0 18px 18px; }
.habit { display: grid; grid-template-columns: auto minmax(0, 1fr) auto auto; align-items: center; gap: 12px; border: 1px solid var(--border); border-radius: 15px; background: #fff; padding: 13px 14px; }
.habit-icon { display: grid; width: 36px; height: 36px; place-items: center; border-radius: 12px; background: var(--accent-soft); color: var(--accent); font-size: 18px; font-weight: 800; }
.habit-name { display: block; overflow-wrap: anywhere; font-size: 14px; font-weight: 750; }
.habit-meta { display: block; margin-top: 3px; color: #837b8c; font-size: 12px; }
.check-in { border: 1px solid var(--border); border-radius: 10px; background: #fff; color: #716a7a; padding: 9px 10px; font-size: 12px; font-weight: 700; }
.check-in.done { border-color: transparent; background: var(--accent); color: #fff; }
.habit-delete { border: 0; border-radius: 8px; background: transparent; color: #9d96a5; padding: 7px; }
.habit-delete:hover { color: #a04343; background: #fff0f0; }
@media (max-width: 650px) {
  .habit-form { grid-template-columns: 1fr 1fr; padding: 16px 20px; }
  .habit-form input { grid-column: 1 / -1; }
  .habit-form button { min-height: 42px; }
  .habit { grid-template-columns: auto minmax(0, 1fr) auto; }
  .habit-delete { grid-column: 3; }
}`

const habitsJS = `(() => {
  "use strict";
  const spec = JSON.parse(document.getElementById("atoms-spec").textContent);
  const preview = window.atomsPreview;
  const form = document.getElementById("habit-form");
  const nameInput = document.getElementById("habit-name");
  const iconSelect = document.getElementById("icon-select");
  const targetSelect = document.getElementById("target-select");
  const habitList = document.getElementById("habit-list");
  const emptyState = document.getElementById("empty-state");
  const dayLabel = document.getElementById("day-label");
  const icons = { check: "✓", heart: "♥", book: "▤", run: "↗", water: "≈" };

  document.documentElement.dataset.theme = spec.theme;
  document.getElementById("app-title").textContent = spec.title;
  document.getElementById("app-description").textContent = spec.description;
  let nextID = 1;
  const state = { habits: spec.initialHabits.map((habit) => ({ ...habit, id: String(nextID++), completedDates: [] })) };

  function today() { return new Date().toISOString().slice(0, 10); }
  function dayBefore(offset) { const day = new Date(); day.setDate(day.getDate() - offset); return day.toISOString().slice(0, 10); }
  function streak(habit) {
    let value = 0;
    for (let offset = 0; offset < 366; offset += 1) {
      if (!habit.completedDates.includes(dayBefore(offset))) break;
      value += 1;
    }
    return value;
  }
  function thisWeek(habit) {
    return habit.completedDates.filter((date) => Array.from({ length: 7 }, (_, offset) => dayBefore(offset)).includes(date)).length;
  }
  function snapshot() {
    return { habits: state.habits.map((habit) => ({ id: habit.id, name: habit.name, icon: habit.icon, targetPerWeek: habit.targetPerWeek, completedDates: [...habit.completedDates] })) };
  }
  function restore(nextState) {
    if (!nextState || !Array.isArray(nextState.habits)) return;
    state.habits = nextState.habits
      .filter((habit) => habit && typeof habit.id === "string" && typeof habit.name === "string" && typeof habit.icon === "string" && Number.isInteger(habit.targetPerWeek) && Array.isArray(habit.completedDates))
      .slice(0, 100)
      .map((habit) => ({ id: habit.id, name: habit.name, icon: habit.icon, targetPerWeek: habit.targetPerWeek, completedDates: habit.completedDates.filter((date) => typeof date === "string").slice(0, 366) }));
    nextID = state.habits.length + 1;
    render();
  }
  function makeHabit(habit) {
    const row = document.createElement("article");
    row.className = "habit";
    const icon = document.createElement("span");
    icon.className = "habit-icon";
    icon.textContent = icons[habit.icon] || "✓";
    const copy = document.createElement("div");
    const name = document.createElement("span");
    name.className = "habit-name";
    name.textContent = habit.name;
    const meta = document.createElement("span");
    meta.className = "habit-meta";
    meta.textContent = "连续 " + streak(habit) + " 天 · 本周 " + thisWeek(habit) + "/" + habit.targetPerWeek;
    copy.append(name, meta);
    const check = document.createElement("button");
    check.className = "check-in" + (habit.completedDates.includes(today()) ? " done" : "");
    check.type = "button";
    check.textContent = habit.completedDates.includes(today()) ? "已打卡" : "今日打卡";
    check.addEventListener("click", () => {
      const key = today();
      habit.completedDates = habit.completedDates.includes(key) ? habit.completedDates.filter((date) => date !== key) : [...habit.completedDates, key].sort();
      render();
    });
    const remove = document.createElement("button");
    remove.className = "habit-delete";
    remove.type = "button";
    remove.textContent = "删除";
    remove.addEventListener("click", () => { state.habits = state.habits.filter((item) => item !== habit); render(); });
    row.append(icon, copy, check, remove);
    return row;
  }
  function render() {
    dayLabel.textContent = "今天 · " + today();
    habitList.replaceChildren(...state.habits.map(makeHabit));
    emptyState.hidden = state.habits.length !== 0;
    preview.publish(snapshot());
  }
  form.addEventListener("submit", (event) => {
    event.preventDefault();
    const name = nameInput.value.trim();
    if (!name) return;
    state.habits.unshift({ id: String(nextID++), name, icon: iconSelect.value, targetPerWeek: Number(targetSelect.value), completedDates: [] });
    nameInput.value = "";
    nameInput.focus();
    render();
  });
  preview.onRestore(restore);
  render();
})();`
