import { logoSVG } from "./logo.js";

const ORDER = ["projects", "board", "run", "plans"];
const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

function copy(s, key) { return s[key] ?? key; }

function shell(s, body) {
  return `<div class="dd-app">
    <header class="dd-topbar"><div class="dd-logo">${logoSVG()}</div><span class="dd-spacer"></span><button class="dd-tool">${copy(s, "desktop.language")}</button><button class="dd-tool">⚙ <span>${copy(s, "desktop.settings")}</span></button></header>
    <main class="dd-main">${body}</main>
  </div>`;
}

function projects(s) {
  return shell(s, `<div class="dd-page-head"><div><h3>${copy(s, "desktop.yourProjects")}</h3><p>${copy(s, "desktop.projectSub")}</p></div><button class="dd-btn dd-primary">+ ${copy(s, "desktop.newProject")}</button></div>
    <section class="dd-groups">
      <div class="dd-group"><button class="dd-group-head">⌄ <strong>${copy(s, "desktop.clientNorth")}</strong><span>${copy(s, "desktop.twoProjects")}</span></button><div class="dd-projects">
        <article class="dd-project"><div><strong>troopa</strong><small>github.com/troopa</small></div><div class="dd-stats"><span>2 ${copy(s, "desktop.ready")}</span><span>1 ${copy(s, "desktop.progress")}</span><span>1 ${copy(s, "desktop.planActive")}</span></div></article>
        <article class="dd-project"><div><strong>landing</strong><small>github.com/north/landing</small></div><div class="dd-stats"><span>4 ${copy(s, "desktop.backlog")}</span><span>2 ${copy(s, "desktop.ready")}</span></div></article>
      </div></div>
      <div class="dd-group"><button class="dd-group-head">› <strong>${copy(s, "desktop.clientStudio")}</strong><span>${copy(s, "desktop.oneProject")}</span></button></div>
    </section>
    <div class="dd-modal-wrap"><div class="dd-modal"><header><strong>${copy(s, "desktop.newProject")}</strong><button>×</button></header><label>${copy(s, "desktop.projectName")}<input value="troopa desktop" readonly></label><label>${copy(s, "desktop.repository")}<input value="~/code/troopa" readonly></label><footer><button class="dd-btn">${copy(s, "desktop.cancel")}</button><button class="dd-btn dd-primary">${copy(s, "desktop.createProject")}</button></footer></div></div>`);
}

function card(s, title, cls = "") { return `<button class="dd-card ${cls}"><strong>${title}</strong><span><i></i>${copy(s, "desktop.high")}</span></button>`; }
function board(s) {
  return shell(s, `<div class="dd-page-head dd-board-head"><div><button class="dd-back">← ${copy(s, "desktop.projects")}</button><h3>troopa</h3></div><button class="dd-btn dd-primary">+ ${copy(s, "desktop.newTask")}</button></div>
    <div class="dd-board"><section class="dd-column"><header>${copy(s, "desktop.backlog")} <b>2</b></header>${card(s, copy(s, "desktop.siteTask"), "dd-moving")}${card(s, copy(s, "desktop.docsTask"))}</section><section class="dd-column dd-ready"><header>${copy(s, "desktop.ready")} <b>1</b></header><div class="dd-drop">${copy(s, "desktop.dropHere")}</div></section><section class="dd-column"><header>${copy(s, "desktop.progress")} <b>1</b></header>${card(s, copy(s, "desktop.testTask"))}</section></div>
    <aside class="dd-detail"><header><strong>${copy(s, "desktop.detail")}</strong><button>×</button></header><h4>${copy(s, "desktop.siteTask")}</h4><p>${copy(s, "desktop.taskDescription")}</p><div class="dd-detail-row"><span>${copy(s, "desktop.status")}</span><b>${copy(s, "desktop.progress")}</b></div><div class="dd-agent">✦ <span>Claude</span><small>${copy(s, "desktop.working")}</small></div></aside><div class="dd-cursor dd-cursor-board">${pointer()}</div>`);
}

function run(s) {
  return shell(s, `<div class="dd-page-head dd-run-head"><div><h3>${copy(s, "desktop.siteTask")}</h3><p><span class="dd-badge">${copy(s, "desktop.review")}</span> feat/desktop-demo</p></div><button class="dd-btn dd-approve">${copy(s, "desktop.approve")}</button></div>
    <div class="dd-run-tabs"><button>${copy(s, "desktop.summary")}</button><button class="dd-changes-tab">${copy(s, "desktop.changes")}</button><button class="active">${copy(s, "desktop.log")}</button></div>
    <div class="dd-log"><p class="dd-log-line">▸ ${copy(s, "desktop.readTask")} <b>✓</b></p><p class="dd-log-line">▸ ${copy(s, "desktop.testCommand")} <b>✓</b></p><p class="dd-log-agent">${copy(s, "desktop.agentMessage")}<span class="dd-caret">▋</span></p></div>
    <div class="dd-diff"><header>src/desktop-demo.js <span>${copy(s, "desktop.comment")}</span></header><code class="dd-del">− const view = oldView</code><code class="dd-add">+ const view = desktopDemo</code><div class="dd-line-comment">${copy(s, "desktop.lineComment")}</div></div><div class="dd-notice">✓ ${copy(s, "desktop.approved")}</div><div class="dd-cursor dd-cursor-run">${pointer()}</div>`);
}

function plans(s) {
  const steps = ["desktop.planOne", "desktop.planTwo", "desktop.planThree", "desktop.planFour"].map((key, i) => `<li class="dd-plan-step step-${i}"><span>${i < 2 ? "✓" : i === 2 ? "•" : "4"}</span><div><strong>${copy(s, key)}</strong><small>${i < 2 ? copy(s, "desktop.done") : i === 2 ? copy(s, "desktop.running") : copy(s, "desktop.waiting")}</small></div></li>`).join("");
  return shell(s, `<div class="dd-page-head"><div><h3>${copy(s, "desktop.plansTitle")}</h3><p>${copy(s, "desktop.plansSub")}</p></div><button class="dd-btn dd-primary">+ ${copy(s, "desktop.newPlan")}</button></div><div class="dd-plan-layout"><aside><button class="dd-plan-select"><strong>${copy(s, "desktop.releasePlan")}</strong><span>2 / 4</span></button><button>${copy(s, "desktop.archivePlan")}</button></aside><section class="dd-plan-detail"><header><div><h4>${copy(s, "desktop.releasePlan")}</h4><p>${copy(s, "desktop.planGoal")}</p></div><span class="dd-badge">${copy(s, "desktop.autoOn")}</span></header><div class="dd-progress"><i></i></div><ol>${steps}</ol></section></div>`);
}

function pointer() { return `<svg viewBox="0 0 30 38" aria-hidden="true"><path d="M3 2v27l7-7 5 12 5-2-5-12h10z"/></svg><i></i>`; }

export function createDesktopDemo({ screenEl, figureEl, getStrings, onSceneChange }) {
  let scene = "projects";
  let token = 0;
  let visible = true;
  const reduced = matchMedia("(prefers-reduced-motion: reduce)");
  const render = () => {
    const s = getStrings();
    screenEl.innerHTML = ({ projects, board, run, plans })[scene](s);
    screenEl.dataset.scene = scene;
    onSceneChange?.(scene);
  };
  async function play(next, loop = true) {
    scene = ORDER.includes(next) ? next : "projects";
    const own = ++token;
    render();
    if (reduced.matches || !visible || document.hidden) return;
    const duration = scene === "run" ? 5500 : 4800;
    await wait(duration);
    if (loop && own === token && visible && !document.hidden) play(ORDER[(ORDER.indexOf(scene) + 1) % ORDER.length]);
  }
  const observer = new IntersectionObserver(([entry]) => { visible = entry.isIntersecting; if (visible && !reduced.matches && !document.hidden) play(scene); }, { threshold: 0.15 });
  observer.observe(figureEl);
  document.addEventListener("visibilitychange", () => { if (!document.hidden && visible && !reduced.matches) play(scene); });
  reduced.addEventListener("change", () => play(scene, false));
  return { play, refresh: () => { ++token; render(); }, current: () => scene };
}
