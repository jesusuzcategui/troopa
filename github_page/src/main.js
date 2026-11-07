import "./style.css";
import { strings } from "./i18n.js";
import { logoSVG } from "./logo.js";
import { createPlayer } from "./terminal.js";
import { renderFlowmap } from "./flowmap.js";

const root = document.documentElement;

// Language: English by default, Spanish when chosen; remembered per browser.
const store = {
  get(k) { try { return localStorage.getItem(k); } catch { return null; } },
  set(k, v) { try { localStorage.setItem(k, v); } catch { /* private mode */ } },
};

function applyLang(lang) {
  const dict = strings[lang] ?? strings.en;
  root.lang = lang;
  root.dataset.lang = lang;
  document.querySelectorAll("[data-i18n]").forEach((el) => {
    const v = dict[el.dataset.i18n];
    if (v) el.innerHTML = v;
  });
  document.getElementById("lang").textContent = lang === "en" ? "ES" : "EN";
  document.querySelectorAll(".copy").forEach((b) => (b.textContent = dict.copy));
  renderFlowmap(document.getElementById("flowmap"), dict);
}

let lang = store.get("troopa-lang") === "es" ? "es" : "en";
applyLang(lang);
document.getElementById("lang").addEventListener("click", () => {
  lang = lang === "en" ? "es" : "en";
  store.set("troopa-lang", lang);
  applyLang(lang);
});

// Theme: follows the system until the visitor picks one.
const savedTheme = store.get("troopa-theme");
if (savedTheme) root.dataset.theme = savedTheme;
document.getElementById("theme").addEventListener("click", () => {
  const dark = root.dataset.theme
    ? root.dataset.theme === "dark"
    : !window.matchMedia("(prefers-color-scheme: light)").matches;
  root.dataset.theme = dark ? "light" : "dark";
  store.set("troopa-theme", root.dataset.theme);
});

// Logo in the header, drawn like the terminal draws it.
document.getElementById("brand").innerHTML = logoSVG();

// Hero terminal: replays client, project, task and agent run; tabs jump to a scene.
const sceneTabs = document.querySelectorAll(".term-tabs [data-scene]");
const player = createPlayer({
  screenEl: document.getElementById("term-screen"),
  keyEl: document.getElementById("term-key"),
  onSceneChange: (name) =>
    sceneTabs.forEach((t) => t.setAttribute("aria-selected", String(t.dataset.scene === name))),
});
sceneTabs.forEach((t) => t.addEventListener("click", () => player.play(t.dataset.scene)));
player.play("client");

// Install tabs.
const tabs = document.querySelectorAll('.tabs [role="tab"]');
tabs.forEach((tab) =>
  tab.addEventListener("click", () => {
    tabs.forEach((t) => t.setAttribute("aria-selected", String(t === tab)));
    document.querySelectorAll(".panel").forEach((p) => (p.hidden = p.dataset.panel !== tab.dataset.tab));
  }),
);

// Copy buttons copy the command without the button label.
document.querySelectorAll("pre.cmd").forEach((pre) => {
  const btn = pre.querySelector(".copy");
  btn.addEventListener("click", async () => {
    const text = [...pre.childNodes].filter((n) => n !== btn).map((n) => n.textContent).join("").trim();
    try {
      await navigator.clipboard.writeText(text);
      btn.textContent = (strings[lang] ?? strings.en).copied;
      setTimeout(() => (btn.textContent = (strings[lang] ?? strings.en).copy), 1500);
    } catch {
      const r = document.createRange();
      r.selectNodeContents(pre);
      const s = getSelection();
      s.removeAllRanges();
      s.addRange(r);
    }
  });
});
