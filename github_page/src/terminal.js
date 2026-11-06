// A small player that replays Troopa screens in the hero terminal.
// Each scene is a list of frames; a frame is a screen (lines with inline
// styles) shown for a while, optionally typing text into one of its lines.
//
// Inline styles: {a|accent} {m|muted} {s|selected} {g|green} {r|red} {y|yellow}

import { logoSVG } from "./logo.js";

const COLS = 64;

const esc = (s) => s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");

function renderLine(line) {
  let html = "";
  let last = 0;
  const re = /\{([amsgry])\|([^}]*)\}/g;
  let m;
  while ((m = re.exec(line))) {
    html += esc(line.slice(last, m.index));
    html += `<span class="t-${m[1]}">${esc(m[2])}</span>`;
    last = re.lastIndex;
  }
  return html + esc(line.slice(last));
}

const pad = (s, n) => (s.length >= n ? s.slice(0, n) : s + " ".repeat(n - s.length));
const bar = (s) => `{s|${pad(" " + s, COLS)}}`;

// ---- screens ----------------------------------------------------------------

const projectsScreen = (rows, status = "Ctrl+K palette  n new  c clients  enter board  ? help") => ({
  logo: true,
  lines: ["{a|Projects}", "", ...rows, "", "", `{m|${status}}`],
});

const clientsScreen = (rows, status) => ({
  lines: ["{a|Troopa}{m|  ·  clients}", "", ...rows, "", "", `{m|${status}}`],
});

const formScreen = (title, fields, hint) => ({
  lines: [`{a|${title}}`, "", ...fields, "", `{m|${hint}}`],
});

function board(cols, { status = "v list  z fold  m status  P priority  A agent  tab panel", sel = "" } = {}) {
  // cols: [[name, [[title, meta]]], …] rendered as three visible columns + a folded strip
  const w = 20;
  const head = cols.map(([n, cards], i) => `{${i === 0 ? "a" : "m"}|${pad(`▾ ${n} ${cards.length}`, w)}}`).join("") + "{m|▸}";
  const lines = ["{a|Troopa}{m|  ·  Landing page}", "", head];
  const depth = Math.max(1, ...cols.map(([, c]) => c.length));
  for (let i = 0; i < depth; i++) {
    let l1 = "";
    let l2 = "";
    cols.forEach(([, cards]) => {
      const c = cards[i];
      if (!c) {
        l1 += " ".repeat(w);
        l2 += " ".repeat(w);
        return;
      }
      const t = pad(" " + c[0], w - 1);
      l1 += (c[0] === sel ? `{s|${t}}` : t) + " ";
      l2 += `{m|${pad(" " + c[1], w)}}`;
    });
    lines.push(l1 + "{m|d}", l2 + "{m|o}", "");
  }
  if (cols.every(([, c]) => c.length === 0)) lines.push(`{m|  · empty}`);
  while (lines.length < 15) lines.push("");
  lines.push(`{m|${status}}`);
  return { lines };
}

function popup(base, title, options, cursor = 0) {
  const w = 44;
  const box = [
    `{a|╭${"─".repeat(w)}╮}`,
    `{a|│} {a|${pad(title, w - 2)}} {a|│}`,
    `{a|│}${" ".repeat(w)}{a|│}`,
    ...options.map((o, i) => `{a|│} ${i === cursor ? `{s|${pad(` ${i + 1}  ${o}`, w - 2)}}` : pad(` ${i + 1}  ${o}`, w - 2)} {a|│}`),
    `{a|│}${" ".repeat(w)}{a|│}`,
    `{a|│} {m|${pad("j/k  enter  1–9  esc", w - 2)}} {a|│}`,
    `{a|╰${"─".repeat(w)}╯}`,
  ];
  const lines = [...base.lines];
  const top = 4;
  box.forEach((b, i) => {
    lines[top + i] = "        " + b;
  });
  return { ...base, lines };
}

// ---- scenes -----------------------------------------------------------------

const projRows = [bar("Landing page           landing-page  ·  Astro  ·  Acme Studio"), "{m|   /home/me/src/landing}"];

const SCENES = {
  client: [
    { screen: projectsScreen(["{m|  No projects yet. Press n to create one.}"]), ms: 1600, key: "c" },
    { screen: clientsScreen(["{m|  No clients yet. Press n to create one.}"], "n new  e edit  a archive  enter board  esc back"), ms: 1300, key: "n" },
    {
      screen: formScreen("New client", ["  Name     ▌", "  Contact", "  Notes"], "enter save  tab field  esc cancel"),
      type: { line: 2, prefix: "  Name     ", text: "Acme Studio" },
      ms: 600,
    },
    {
      screen: formScreen("New client", ["  Name     Acme Studio", "  Contact  ▌", "  Notes"], "enter save  tab field  esc cancel"),
      type: { line: 3, prefix: "  Contact  ", text: "hello@acme.example" },
      ms: 800,
      key: "enter",
    },
    { screen: clientsScreen([bar("Acme Studio            hello@acme.example")], "Created: Acme Studio"), ms: 2400 },
  ],
  project: [
    { screen: projectsScreen(["{m|  No projects yet. Press n to create one.}"]), ms: 1300, key: "n" },
    {
      screen: formScreen("New project", ["  Name   ▌", "  Slug", "  Path", "  Stack", "  Client ‹ Acme Studio ›"], "enter save  tab field  ←/→ client  esc cancel"),
      type: { line: 2, prefix: "  Name   ", text: "Landing page" },
      ms: 300,
    },
    {
      screen: formScreen("New project", ["  Name   Landing page", "  Slug   landing-page", "  Path   ▌", "  Stack", "  Client ‹ Acme Studio ›"], "enter save  tab field  ←/→ client  esc cancel"),
      type: { line: 4, prefix: "  Path   ", text: "~/src/landing" },
      ms: 300,
    },
    {
      screen: formScreen("New project", ["  Name   Landing page", "  Slug   landing-page", "  Path   ~/src/landing", "  Stack  ▌", "  Client ‹ Acme Studio ›"], "enter save  tab field  ←/→ client  esc cancel"),
      type: { line: 5, prefix: "  Stack  ", text: "Astro" },
      ms: 700,
      key: "enter",
    },
    {
      screen: formScreen("Confirm", ["  The path /home/me/src/landing does not exist.", "  Create the folder, git init -b main, a README", "  and an initial commit?"], "y yes · n save without init · esc back"),
      ms: 1700,
      key: "y",
    },
    { screen: projectsScreen(projRows, "Git repo created in /home/me/src/landing"), ms: 2600 },
  ],
  task: [
    { screen: board([["inbox", []], ["backlog", []], ["ready", []]]), ms: 1200, key: "n" },
    {
      screen: { lines: ["{a|New task}", "", "  Title  ▌", "", "{m|enter create  esc cancel}"] },
      type: { line: 2, prefix: "  Title  ", text: "Add a contact form" },
      ms: 500,
      key: "enter",
    },
    { screen: board([["inbox", [["Add a contact form", "medium · Claude Code"]]], ["backlog", []], ["ready", []]], { sel: "Add a contact form", status: "Created: Add a contact form" }), ms: 1300, key: "e" },
    {
      screen: formScreen(
        "Edit task",
        ["  Title        Add a contact form", "  Description  ▌", "", "  Agent        Claude Code  ←/→", "  Mode         yolo  ←/→", "  Commits      auto  ←/→", "  Type         feature  ←/→"],
        "ctrl+s save  tab field  ←/→ options  esc cancel",
      ),
      type: { line: 3, prefix: "  Description  ", text: "Name, email and message; send with fetch." },
      ms: 800,
      key: "ctrl+s",
    },
    { screen: board([["inbox", [["Add a contact form", "medium · Claude Code"]]], ["backlog", []], ["ready", []]], { sel: "Add a contact form", status: "Saved: Add a contact form" }), ms: 900, key: "P" },
    {
      screen: popup(board([["inbox", [["Add a contact form", "medium · Claude Code"]]], ["backlog", []], ["ready", []]], { sel: "Add a contact form" }), "Priority · Add a contact form", ["low", "medium", "high"], 2),
      ms: 1300,
      key: "3",
    },
    { screen: board([["inbox", [["Add a contact form", "high · Claude Code"]]], ["backlog", []], ["ready", []]], { sel: "Add a contact form", status: "Priority: high" }), ms: 2200 },
  ],
  run: [
    { screen: board([["inbox", []], ["backlog", [["Add a contact form", "high · Claude Code"]]], ["ready", []]], { sel: "Add a contact form" }), ms: 1200, key: "]" },
    {
      screen: popup(board([["inbox", []], ["backlog", [["Add a contact form", "high · Claude Code"]]], ["ready", []]], { sel: "Add a contact form" }), "Launch Claude Code (yolo) on «Add a…»?", ["Yes, launch", "No"]),
      ms: 1500,
      key: "enter",
    },
    {
      stream: {
        head: ["{a|Agent}{m|  ·  feature/add-a-contact-form  ·  running}", ""],
        lines: [
          "Reading TASK.md",
          "{m|> Read src/pages/index.astro}",
          "{m|> Write src/components/ContactForm.astro}",
          "{m|> Edit src/pages/index.astro}",
          "{m|> Bash npm test}",
          "{g|✓ 12 tests passed}",
          "{m|> Write SUMMARY.md}",
          "Done.",
        ],
        foot: "{m|j/k scroll  o editor  esc board}",
      },
      ms: 900,
    },
    {
      screen: {
        lines: [
          "{a|Review}{m|  ·  feature/add-a-contact-form  ·  review}",
          "",
          "{m|Summary}",
          "How to run: npm install && npm run dev → /#contact",
          "",
          "{m|Checks}",
          "{g|✓ npm test}",
          "",
          "{a|src/components/ContactForm.astro}{m|  1/2}",
          "{y|@@ -0,0 +1,24 @@}",
          "{g|+<form class=\"contact\" method=\"post\">}",
          "{g|+  <input name=\"email\" type=\"email\" required>}",
          "{g|+  <textarea name=\"message\" required></textarea>}",
          "{g|+</form>}",
          "",
          "{m|y approve  x request changes  d discard  n/p file  o editor}",
        ],
      },
      ms: 2600,
      key: "y",
    },
    { screen: board([["review", []], ["blocked", []], ["done", [["Add a contact form", "high · Claude Code"]]]], { sel: "Add a contact form", status: "Approved. Local branch feature/add-a-contact-form" }), ms: 1300, key: "g" },
    {
      screen: popup(board([["review", []], ["blocked", []], ["done", [["Add a contact form", "high · Claude Code"]]]], { sel: "Add a contact form" }), "Merge into main (squash)?", ["Yes, merge", "No"]),
      ms: 1400,
      key: "enter",
    },
    { screen: board([["review", []], ["blocked", []], ["done", [["Add a contact form", "high · Claude Code"]]]], { sel: "Add a contact form", status: "Merged feature/add-a-contact-form into main as 3f2c1a9" }), ms: 3000 },
  ],
};

export const SCENE_ORDER = ["client", "project", "task", "run"];

// ---- player -----------------------------------------------------------------

export function createPlayer({ screenEl, keyEl, onSceneChange }) {
  const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  let scene = SCENE_ORDER[0];
  let timer = 0;
  let token = 0;

  const wait = (ms, t) => new Promise((res) => { timer = setTimeout(() => res(t === token), ms); });

  function draw(screen, override) {
    const lines = [...screen.lines];
    if (override) lines[override.line] = override.text;
    const body = lines.map((l) => `<div class="t-row">${renderLine(l) || "&nbsp;"}</div>`).join("");
    screenEl.innerHTML = (screen.logo ? `<div class="t-logo">${logoSVG()}</div>` : "") + body;
  }

  function showKey(k) {
    if (!keyEl) return;
    keyEl.textContent = k;
    keyEl.classList.remove("on");
    void keyEl.offsetWidth; // restart the animation
    keyEl.classList.add("on");
  }

  async function playFrame(f, t) {
    if (f.stream) {
      const shown = [...f.stream.head];
      const frame = () => ({ lines: [...shown, ...Array(Math.max(0, 15 - shown.length)).fill(""), f.stream.foot] });
      draw(frame());
      for (const l of f.stream.lines) {
        if (!(await wait(reduced ? 0 : 520, t))) return false;
        shown.push(l);
        draw(frame());
      }
    } else if (f.type) {
      const { line, prefix, text } = f.type;
      for (let i = 0; i <= text.length; i++) {
        draw(f.screen, { line, text: prefix + text.slice(0, i) + "▌" });
        if (!(await wait(reduced ? 0 : 55, t))) return false;
      }
    } else {
      draw(f.screen);
    }
    if (!(await wait(reduced ? 400 : f.ms, t))) return false;
    if (f.key) {
      showKey(f.key);
      if (!(await wait(reduced ? 0 : 350, t))) return false;
    }
    return true;
  }

  async function play(name, { loop = true } = {}) {
    clearTimeout(timer);
    const t = ++token;
    scene = name;
    onSceneChange?.(name);
    for (const f of SCENES[name]) {
      if (!(await playFrame(f, t))) return;
    }
    if (loop && t === token) {
      const next = SCENE_ORDER[(SCENE_ORDER.indexOf(name) + 1) % SCENE_ORDER.length];
      play(next);
    }
  }

  return { play, current: () => scene };
}
