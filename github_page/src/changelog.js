import "./style.css";
import { strings } from "./i18n.js";
import { logoSVG } from "./logo.js";
import changelog from "../../CHANGELOG.md?raw";

const root = document.documentElement;
const store = {
  get(k) { try { return localStorage.getItem(k); } catch { return null; } },
  set(k, v) { try { localStorage.setItem(k, v); } catch { /* private mode */ } },
};

const esc = (s) => s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");

// Inline Markdown used by scripts/changelog.sh: `code` and [text](url).
function inline(s) {
  return esc(s)
    .replace(/`([^`]+)`/g, "<code>$1</code>")
    .replace(/\[([^\]]+)\]\((https?:[^)\s]+)\)/g, '<a href="$2">$1</a>');
}

const slug = (s) => s.toLowerCase().replace(/[^a-z0-9.]+/g, "-").replace(/^-|-$/g, "");

// Turns CHANGELOG.md into sections and fills the version list.
function render() {
  const out = [];
  const toc = [];
  let list = false;
  let section = false;
  const close = () => {
    if (list) out.push("</ul>");
    list = false;
  };
  for (const line of changelog.split("\n")) {
    if (line.startsWith("# ") || line.startsWith("Every version")) continue;
    if (line.startsWith("## ")) {
      close();
      if (section) out.push("</section>");
      section = true;
      const title = line.slice(3);
      const [name, date] = title.split(" · ");
      const id = slug(name);
      const label = name === "Unreleased" ? `<span data-i18n="log.unreleased">Unreleased</span>` : esc(name);
      toc.push(`<a href="#${id}">${label}</a>`);
      out.push(`<section id="${id}"><h2>${label}${date ? ` <small>${esc(date)}</small>` : ""}</h2>`);
    } else if (line.startsWith("### ")) {
      close();
      const group = line.slice(4);
      out.push(`<h3 data-i18n="log.${slug(group)}">${esc(group)}</h3>`);
    } else if (line.startsWith("- ")) {
      if (!list) out.push("<ul>");
      list = true;
      out.push(`<li>${inline(line.slice(2))}</li>`);
    }
  }
  close();
  if (section) out.push("</section>");
  document.getElementById("log").innerHTML = out.join("\n");
  document.getElementById("versions").insertAdjacentHTML("beforeend", toc.join(""));
}

function applyLang(lang) {
  const dict = strings[lang] ?? strings.en;
  root.lang = lang;
  root.dataset.lang = lang;
  document.querySelectorAll("[data-i18n]").forEach((el) => {
    const v = dict[el.dataset.i18n];
    if (v) el.innerHTML = v;
  });
  document.getElementById("lang").textContent = lang === "en" ? "ES" : "EN";
  document.title = lang === "es" ? "Cambios de Troopa" : "Troopa changelog";
}

render();
let lang = store.get("troopa-lang") === "es" ? "es" : "en";
applyLang(lang);
document.getElementById("lang").addEventListener("click", () => {
  lang = lang === "en" ? "es" : "en";
  store.set("troopa-lang", lang);
  applyLang(lang);
});

const savedTheme = store.get("troopa-theme");
if (savedTheme) root.dataset.theme = savedTheme;
document.getElementById("theme").addEventListener("click", () => {
  const dark = root.dataset.theme
    ? root.dataset.theme === "dark"
    : !window.matchMedia("(prefers-color-scheme: light)").matches;
  root.dataset.theme = dark ? "light" : "dark";
  store.set("troopa-theme", root.dataset.theme);
});

document.getElementById("brand").innerHTML = logoSVG();
