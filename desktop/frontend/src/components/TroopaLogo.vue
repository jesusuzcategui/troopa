<script setup lang="ts">
// Mirrors github_page/src/logo.js so the desktop app carries the same mark.
const ROWS = [
  "   ▲      ▲      ▲",
  " [◉_◉]  [◉_◉]  [◉_◉]     ▀█▀ █▀█ █▀█ █▀█ █▀█ ▄▀█",
  " /|█|\\  /|█|\\  /|█|\\      █  █▀▄ █▄█ █▄█ █▀▀ █▀█",
  "  / \\    / \\    / \\",
];

const CELL_W = 12;
const CELL_H = 24;
const WORDMARK_FROM = 25;

function role(row: number, col: number): string {
  if (col >= WORDMARK_FROM) return "accent";
  if (col >= 7 && col < 14) return "accent";
  if (row === 0) return "accent";
  if (row === 1) return "body";
  return "dim";
}

const cols = Math.max(...ROWS.map((r) => [...r].length));
const width = cols * CELL_W;
const height = ROWS.length * CELL_H;
const half = CELL_H / 2;

interface Cell {
  kind: "rect" | "text";
  cls: string;
  x: number;
  y: number;
  w?: number;
  h?: number;
  ch?: string;
}

const cells: Cell[] = [];
ROWS.forEach((line, r) => {
  [...line].forEach((ch, c) => {
    if (ch === " ") return;
    const x = c * CELL_W;
    const y = r * CELL_H;
    const cls = `lg-${role(r, c)}`;
    if (ch === "█") {
      cells.push({ kind: "rect", cls, x, y, w: CELL_W, h: CELL_H });
    } else if (ch === "▀") {
      cells.push({ kind: "rect", cls, x, y, w: CELL_W, h: half });
    } else if (ch === "▄") {
      cells.push({ kind: "rect", cls, x, y: y + half, w: CELL_W, h: half });
    } else {
      cells.push({ kind: "text", cls, x: x + CELL_W / 2, y: y + CELL_H * 0.74, ch });
    }
  });
});
</script>

<template>
  <svg
    class="logo-svg"
    :viewBox="`0 0 ${width} ${height}`"
    role="img"
    aria-label="Troopa"
    xmlns="http://www.w3.org/2000/svg"
  >
    <template v-for="(cell, idx) in cells" :key="idx">
      <rect
        v-if="cell.kind === 'rect'"
        :class="cell.cls"
        :x="cell.x"
        :y="cell.y"
        :width="cell.w"
        :height="cell.h"
      />
      <text v-else :class="cell.cls" :x="cell.x" :y="cell.y">{{ cell.ch }}</text>
    </template>
  </svg>
</template>
