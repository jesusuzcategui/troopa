import type { Palette } from "../bindings/github.com/jesusuzcategui/troopa/internal/theme/models";

function parseHex(hex: string): [number, number, number] | null {
  const clean = hex.trim().replace(/^#/, "");
  if (clean.length === 3) {
    const r = parseInt(clean[0] + clean[0], 16);
    const g = parseInt(clean[1] + clean[1], 16);
    const b = parseInt(clean[2] + clean[2], 16);
    if (Number.isNaN(r) || Number.isNaN(g) || Number.isNaN(b)) return null;
    return [r, g, b];
  }
  if (clean.length !== 6) return null;
  const r = parseInt(clean.slice(0, 2), 16);
  const g = parseInt(clean.slice(2, 4), 16);
  const b = parseInt(clean.slice(4, 6), 16);
  if (Number.isNaN(r) || Number.isNaN(g) || Number.isNaN(b)) return null;
  return [r, g, b];
}

function toHex(r: number, g: number, b: number): string {
  const clamp = (n: number) => Math.max(0, Math.min(255, Math.round(n)));
  return (
    "#" +
    [clamp(r), clamp(g), clamp(b)]
      .map((n) => n.toString(16).padStart(2, "0"))
      .join("")
  );
}

function mixHex(a: string, b: string, weightB: number): string {
  const pa = parseHex(a);
  const pb = parseHex(b);
  if (!pa || !pb) return a;
  const w = Math.max(0, Math.min(1, weightB));
  return toHex(
    pa[0] * (1 - w) + pb[0] * w,
    pa[1] * (1 - w) + pb[1] * w,
    pa[2] * (1 - w) + pb[2] * w
  );
}

function contrastingInk(bgHex: string): string {
  const rgb = parseHex(bgHex);
  if (!rgb) return "#ffffff";
  const lum = 0.299 * rgb[0] + 0.587 * rgb[1] + 0.114 * rgb[2];
  return lum > 140 ? "#11111b" : "#ffffff";
}

export function themeVars(p: Palette): Record<string, string> {
  const c = p.colors;
  const raised = p.light
    ? mixHex(c.bg, "#ffffff", 0.65)
    : mixHex(c.bg, c.selected, 0.72);
  const strong = p.light
    ? mixHex(c.fg, "#000000", 0.45)
    : mixHex(c.fg, "#ffffff", 0.45);
  const line = mixHex(c.bg, c.fg, p.light ? 0.18 : 0.2);

  return {
    "--bg": c.bg,
    "--band": c.status,
    "--raised": raised,
    "--selected": c.selected,
    "--fg": c.fg,
    "--strong": strong,
    "--muted": c.muted,
    "--line": line,
    "--accent": c.accent,
    "--accent-ink": contrastingInk(c.accent),
    "--ok": c.diff_add,
    "--peach": c.diff_hunk,
    "--danger": c.error,
    "--danger-ink": contrastingInk(c.error),
    "--diff-add": c.diff_add,
    "--diff-del": c.diff_del,
    "--diff-hunk": c.diff_hunk,
    "--term-bg": c.status,
    "color-scheme": p.light ? "light" : "dark",
  };
}

export function applyPalette(p: Palette): void {
  const root = document.documentElement;
  const vars = themeVars(p);
  for (const [k, v] of Object.entries(vars)) {
    if (k === "color-scheme") {
      root.style.colorScheme = v;
    } else {
      root.style.setProperty(k, v);
    }
  }
  root.dataset.theme = p.light ? "light" : "dark";
  root.dataset.themeName = p.name;
}

export function findTheme(themes: Palette[], name: string): Palette | undefined {
  const clean = name.trim();
  return (
    themes.find((th) => th.name === clean) ??
    themes.find((th) => th.name === "dark") ??
    themes[0]
  );
}
