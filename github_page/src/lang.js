const key = "troopa-lang";

const storedLanguage = () => {
  try { return localStorage.getItem(key); } catch { return null; }
};

export function initialLanguage() {
  const requested = new URLSearchParams(location.search).get("lang");
  if (requested === "en" || requested === "es") return requested;
  const saved = storedLanguage();
  if (saved === "en" || saved === "es") return saved;
  return navigator.language?.toLowerCase().startsWith("es") ? "es" : "en";
}

export function saveLanguage(lang) {
  try { localStorage.setItem(key, lang); } catch { /* private mode */ }
}

export function replaceLanguageInURL(lang) {
  const url = new URL(location.href);
  url.searchParams.set("lang", lang);
  history.replaceState(null, "", `${url.pathname}${url.search}${url.hash}`);
}

// Keep relative links shareable in the selected language while leaving external
// destinations and same-page anchors alone.
export function preserveLanguageLinks(lang) {
  document.querySelectorAll("a[href]").forEach((link) => {
    const href = link.getAttribute("href");
    if (!href || href.startsWith("#")) return;
    const url = new URL(href, location.href);
    if (url.origin !== location.origin) return;
    url.searchParams.set("lang", lang);
    link.href = `${url.pathname}${url.search}${url.hash}`;
  });
}
