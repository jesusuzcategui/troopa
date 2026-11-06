// The Troopa logo drawn exactly like the terminal draws it: a grid of cells.
// Block characters (▀ ▄ █) become rectangles that fill the cell, so the
// wordmark has no gaps; every other character is text centered in its cell.

// The wordmark sits on the two middle rows, vertically centered on the robots.
const ROWS = [
  "   ▲      ▲      ▲",
  " [◉_◉]  [◉_◉]  [◉_◉]     ▀█▀ █▀█ █▀█ █▀█ █▀█ ▄▀█",
  " /|█|\\  /|█|\\  /|█|\\      █  █▀▄ █▄█ █▄█ █▀▀ █▀█",
  "  / \\    / \\    / \\",
];

const CELL_W = 12;
const CELL_H = 24;
const WORDMARK_FROM = 25; // first column of the wordmark

// Color role of a cell: accent (antennas, leader, wordmark), body (heads) or dim (bodies).
function role(row, col) {
  if (col >= WORDMARK_FROM) return "accent";
  if (col >= 7 && col < 14) return "accent"; // the leader in the middle
  if (row === 0) return "accent";
  if (row === 1) return "body";
  return "dim";
}

const esc = (s) => s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");

/**
 * Builds the logo as an SVG string.
 * @param {{tagline?: string, compact?: boolean}} opts compact drops the robots' legs.
 */
export function logoSVG({ tagline = "", compact = false } = {}) {
  const rows = compact ? ROWS.slice(0, 2) : ROWS;
  const cols = Math.max(...ROWS.map((r) => [...r].length));
  const width = cols * CELL_W;
  const height = rows.length * CELL_H;
  const parts = [];
  rows.forEach((line, r) => {
    [...line].forEach((ch, c) => {
      if (ch === " ") return;
      const x = c * CELL_W;
      const y = r * CELL_H;
      const cls = `lg-${role(r, c)}`;
      const half = CELL_H / 2;
      if (ch === "█") parts.push(`<rect class="${cls}" x="${x}" y="${y}" width="${CELL_W}" height="${CELL_H}"/>`);
      else if (ch === "▀") parts.push(`<rect class="${cls}" x="${x}" y="${y}" width="${CELL_W}" height="${half}"/>`);
      else if (ch === "▄") parts.push(`<rect class="${cls}" x="${x}" y="${y + half}" width="${CELL_W}" height="${half}"/>`);
      else parts.push(`<text class="${cls}" x="${x + CELL_W / 2}" y="${y + CELL_H * 0.74}">${esc(ch)}</text>`);
    });
  });
  if (tagline && !compact) {
    parts.push(`<text class="lg-dim lg-tag" x="${WORDMARK_FROM * CELL_W}" y="${3 * CELL_H + CELL_H * 0.74}">${esc(tagline)}</text>`);
  }
  return `<svg class="logo-svg" viewBox="0 0 ${width} ${height}" role="img" aria-label="Troopa" xmlns="http://www.w3.org/2000/svg">${parts.join("")}</svg>`;
}
