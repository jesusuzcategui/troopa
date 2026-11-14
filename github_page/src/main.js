import "./style.css";
import { strings } from "./i18n.js";
import { logoSVG } from "./logo.js";
import { initialLanguage, preserveLanguageLinks, replaceLanguageInURL, saveLanguage } from "./lang.js";
import { createPlayer } from "./terminal.js";
import { createDesktopDemo } from "./desktop-demo.js";
import { renderFlowmap } from "./flowmap.js";

const root = document.documentElement;
let desktopDemo;

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
  document.querySelectorAll("[data-i18n-aria]").forEach((el) => {
    const v = dict[el.dataset.i18nAria];
    if (v) el.setAttribute("aria-label", v);
  });
  document.getElementById("lang").textContent = lang === "en" ? "ES" : "EN";
  document.querySelectorAll(".copy").forEach((b) => (b.textContent = dict.copy));
  preserveLanguageLinks(lang);
  renderFlowmap(document.getElementById("flowmap"), dict);
  desktopDemo?.refresh();
}

let lang = initialLanguage();
applyLang(lang);
document.getElementById("lang").addEventListener("click", () => {
  lang = lang === "en" ? "es" : "en";
  saveLanguage(lang);
  replaceLanguageInURL(lang);
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

// Desktop app: the illustration uses the same language as the downloadable app.
const desktopTabs = document.querySelectorAll(".desktop-demo-tabs [data-desktop-scene]");
desktopDemo = createDesktopDemo({
  screenEl: document.getElementById("desktop-demo-screen"),
  figureEl: document.querySelector(".desktop-demo"),
  getStrings: () => strings[lang] ?? strings.en,
  onSceneChange: (name) => desktopTabs.forEach((tab) => tab.setAttribute("aria-selected", String(tab.dataset.desktopScene === name))),
});
desktopTabs.forEach((tab) => tab.addEventListener("click", () => desktopDemo.play(tab.dataset.desktopScene)));
// A scene can be linked directly for documentation captures; normal visitors
// start at Projects and continue through the loop.
desktopDemo.play(new URLSearchParams(location.search).get("desktop-scene") ?? "projects");

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

// Download buttons: GitHub's asset names carry the version, so they are asked
// for at load time. Without an answer (offline, rate limited) the buttons keep
// pointing at the releases page.
const DOWNLOADS = {
  "dl-linux-appimage": "_linux_amd64.AppImage",
  "dl-linux-deb": "_linux_amd64.deb",
  "dl-macos": "_darwin_universal.app.zip",
  "dl-windows": "_windows_amd64_installer.exe",
};

async function latestRelease() {
  const api = "https://api.github.com/repos/jesusuzcategui/troopa/releases";
  try {
    const res = await fetch(`${api}?per_page=20`);
    if (!res.ok) return null;
    const releases = await res.json();
    // Prefer a stable release, but keep the newest pre-release usable too.
    return releases.find((release) => !release.prerelease && !release.draft && release.assets?.length)
      ?? releases.find((release) => !release.draft && release.assets?.length)
      ?? null;
  } catch {
    return null;
  }
}

latestRelease().then((release) => {
  if (!release) return;
  for (const [id, suffix] of Object.entries(DOWNLOADS)) {
    // The prefix matters: the CLI ships assets with the same suffixes.
    const asset = release.assets.find((a) => a.name.startsWith("troopa-desktop_") && a.name.endsWith(suffix));
    const el = document.getElementById(id);
    if (asset && el) el.href = asset.browser_download_url;
  }
  document.getElementById("dl-version").textContent = release.tag_name;
});
