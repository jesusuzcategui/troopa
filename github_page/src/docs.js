import "./style.css";
import { strings } from "./i18n.js";
import { logoSVG } from "./logo.js";
import { initialLanguage, preserveLanguageLinks, replaceLanguageInURL, saveLanguage } from "./lang.js";

const root = document.documentElement;
const store = {
  get(k) { try { return localStorage.getItem(k); } catch { return null; } },
  set(k, v) { try { localStorage.setItem(k, v); } catch { /* private mode */ } },
};

// The page holds both languages; the TOC points at the visible copy.
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
  preserveLanguageLinks(lang);
  document.querySelectorAll(".toc a").forEach((a) => {
    const id = a.getAttribute("href").replace(/^#/, "").replace(/-es$/, "");
    a.setAttribute("href", "#" + (lang === "es" ? id + "-es" : id));
  });
  document.title = lang === "es" ? "Documentación de Troopa" : "Troopa docs";
}

let lang = initialLanguage();
applyLang(lang);
document.getElementById("lang").addEventListener("click", () => {
  lang = lang === "en" ? "es" : "en";
  saveLanguage(lang);
  replaceLanguageInURL(lang);
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

document.querySelectorAll("pre.cmd").forEach((pre) => {
  const btn = pre.querySelector(".copy");
  if (!btn) return;
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

// Highlight the section in view.
const links = [...document.querySelectorAll(".toc a")];
const observer = new IntersectionObserver(
  (entries) => {
    entries.forEach((e) => {
      if (!e.isIntersecting) return;
      links.forEach((a) => a.classList.toggle("on", a.getAttribute("href") === "#" + e.target.id));
    });
  },
  { rootMargin: "-80px 0px -70% 0px" },
);
document.querySelectorAll(".doc section[id]").forEach((s) => observer.observe(s));
