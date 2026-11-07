// The animated map under the hero: a task travels from the inbox to main.
// One timeline drives two layouts (wide for desktop, tall for phones); CSS
// shows the one that fits. Everything is SMIL inside the SVG, so it needs no
// timers and stays in sync. With reduced motion the map is drawn static.

const NS = "http://www.w3.org/2000/svg";
const LOOP = 15; // seconds per cycle

// Stops of the task, in order. "phone" and the loops are drawn apart.
const STOPS = ["task", "plan", "tree", "agent", "checks", "review", "main"];

const LAYOUTS = {
  wide: {
    w: 1120, h: 430, box: [140, 76],
    at: { task: [80, 250], plan: [240, 250], tree: [400, 250], agent: [560, 250], checks: [720, 250], review: [880, 250], main: [1040, 250] },
    phone: { x: 820, y: 14, w: 120, h: 176 },
    link: "M 880 212 L 880 190",
    // Request changes: review → agent. Next step: review → worktree, lower.
    changes: "M 880 288 C 880 350, 560 350, 560 288",
    changesLabel: { x: 720, y: 324 },
    next: "M 850 288 C 850 430, 400 430, 400 288",
    nextLabel: { x: 625, y: 415 },
  },
  tall: {
    w: 360, h: 850, box: [176, 62],
    at: { task: [180, 45], plan: [180, 145], tree: [180, 245], agent: [180, 345], checks: [180, 445], review: [180, 545], main: [180, 800] },
    phone: { x: 196, y: 606, w: 128, h: 160 },
    link: "M 250 576 L 250 606",
    changes: "M 92 555 C 40 555, 40 345, 92 345",
    changesLabel: { x: 70, y: 450, rotate: true },
    next: "M 92 535 C 0 535, 0 245, 92 245",
    nextLabel: { x: 33, y: 390, rotate: true, short: true },
  },
};

function el(name, attrs = {}, parent) {
  const n = document.createElementNS(NS, name);
  for (const [k, v] of Object.entries(attrs)) n.setAttribute(k, v);
  if (parent) parent.appendChild(n);
  return n;
}

function text(parent, x, y, str, cls, anchor = "middle") {
  const t = el("text", { x, y, class: cls, "text-anchor": anchor }, parent);
  t.textContent = str;
  return t;
}

// Timeline: hold at each stop, travel between; the review stop holds longer
// while the phone gets the alert and the approve button is tapped.
function timeline() {
  const hold = 0.7, travel = 0.75, reviewHold = 3.4, endHold = 1.6;
  const arrive = {}, leave = {};
  let t = 0.3;
  STOPS.forEach((s, i) => {
    arrive[s] = t;
    t += s === "review" ? reviewHold : s === "main" ? endHold : hold;
    leave[s] = t;
    if (i < STOPS.length - 1) t += travel;
  });
  const scale = (LOOP - 0.4) / t;
  for (const s of STOPS) { arrive[s] *= scale; leave[s] *= scale; }
  return { arrive, leave };
}

const k = (sec) => Math.min(1, Math.max(0, sec / LOOP)).toFixed(4);

function animate(node, attr, points) {
  // points: [[seconds, value], …] sorted; padded to cover 0 and LOOP.
  // Clamp to the loop and drop any point that would go back in time.
  const pts = [];
  for (const [t, v] of points) {
    const c = Math.min(LOOP, Math.max(0, t));
    if (!pts.length || c > pts[pts.length - 1][0]) pts.push([c, v]);
  }
  if (pts[0][0] > 0) pts.unshift([0, pts[0][1]]);
  if (pts[pts.length - 1][0] < LOOP) pts.push([LOOP, pts[pts.length - 1][1]]);
  el("animate", {
    attributeName: attr, dur: `${LOOP}s`, repeatCount: "indefinite", calcMode: "linear",
    keyTimes: pts.map((p) => k(p[0])).join(";"), values: pts.map((p) => p[1]).join(";"),
  }, node);
}

function build(layoutName, d, still) {
  const L = LAYOUTS[layoutName];
  const { arrive, leave } = timeline();
  const svg = el("svg", {
    viewBox: `0 0 ${L.w} ${L.h}`, class: `flowmap-svg flowmap-${layoutName}`,
    role: "img", "aria-label": d["map.aria"], preserveAspectRatio: "xMidYMid meet",
  });
  const defs = el("defs", {}, svg);
  const glow = el("filter", { id: `fm-glow-${layoutName}`, x: "-50%", y: "-50%", width: "200%", height: "200%" }, defs);
  el("feGaussianBlur", { stdDeviation: "4" }, glow);
  const marker = el("marker", { id: `fm-arrow-${layoutName}`, viewBox: "0 0 10 10", refX: "8", refY: "5", markerWidth: "7", markerHeight: "7", orient: "auto-start-reverse" }, defs);
  el("path", { d: "M 0 0 L 10 5 L 0 10 z", class: "fm-arrowhead" }, marker);

  const [bw, bh] = L.box;
  // The token rides the top edge of each box so it never covers the labels.
  const pts = STOPS.map((s) => [L.at[s][0], L.at[s][1] - bh / 2]);

  // Rail between stops.
  el("polyline", { points: pts.map((p) => p.join(",")).join(" "), class: "fm-rail" }, svg);

  // Loops back: request changes and next plan step.
  const changes = el("path", { d: L.changes, class: "fm-loop fm-loop-changes", "marker-end": `url(#fm-arrow-${layoutName})` }, svg);
  const next = el("path", { d: L.next, class: "fm-loop fm-loop-next", "marker-end": `url(#fm-arrow-${layoutName})` }, svg);
  for (const [lab, key, cls] of [[L.changesLabel, "map.loopChanges", "fm-loop-label"], [L.nextLabel, L.nextLabel.short ? "map.loopNextShort" : "map.loopNext", "fm-loop-label fm-loop-label-next"]]) {
    const t = text(svg, 0, 0, d[key], cls);
    t.setAttribute("transform", `translate(${lab.x} ${lab.y})` + (lab.rotate ? " rotate(-90)" : ""));
  }
  if (!still) {
    for (const loop of [changes, next]) {
      el("animate", { attributeName: "stroke-dashoffset", from: "24", to: "0", dur: "1.2s", repeatCount: "indefinite" }, loop);
    }
  }

  // Stops.
  STOPS.forEach((s, i) => {
    const [cx, cy] = L.at[s];
    const g = el("g", { class: `fm-stop fm-${s}` }, svg);
    el("rect", { x: cx - bw / 2, y: cy - bh / 2, width: bw, height: bh, rx: 12, class: "fm-box" }, g);
    const lit = el("rect", { x: cx - bw / 2, y: cy - bh / 2, width: bw, height: bh, rx: 12, class: "fm-lit", opacity: still ? 1 : 0 }, g);
    text(g, cx - bw / 2 + 12, cy - bh / 2 + 18, String(i + 1).padStart(2, "0"), "fm-num", "start");
    text(g, cx, cy + (layoutName === "wide" ? 2 : 3), d[`map.${s}`], "fm-title");
    text(g, cx, cy + (layoutName === "wide" ? 22 : 21), d[`map.${s}.sub`], "fm-sub");
    if (!still) {
      const a = arrive[s], l = leave[s];
      animate(lit, "opacity", [[a - 0.05, 0], [a + 0.15, 1], [l, 1], [l + 0.3, 0.35], [LOOP - 0.35, 0.35], [LOOP - 0.05, 0]]);
    }
  });

  // Phone with the Telegram alert.
  const P = L.phone;
  const link = L.link;
  const linkPath = el("path", { d: link, class: "fm-link" }, svg);
  const phone = el("g", { class: "fm-phone" }, svg);
  el("rect", { x: P.x, y: P.y, width: P.w, height: P.h, rx: 16, class: "fm-phone-body" }, phone);
  el("rect", { x: P.x + 7, y: P.y + 18, width: P.w - 14, height: P.h - 30, rx: 6, class: "fm-phone-screen" }, phone);
  el("rect", { x: P.x + P.w / 2 - 14, y: P.y + 7, width: 28, height: 4, rx: 2, class: "fm-phone-notch" }, phone);
  text(phone, P.x + P.w / 2, P.y + 36, "Telegram", "fm-phone-app");

  const bubble = el("g", { class: "fm-bubble", opacity: still ? 1 : 0 }, phone);
  const bx = P.x + 13, bwid = P.w - 26;
  const by = P.y + 46;
  el("rect", { x: bx, y: by, width: bwid, height: 34, rx: 6, class: "fm-bubble-bg" }, bubble);
  text(bubble, bx + 7, by + 14, d["map.alert"], "fm-bubble-title", "start");
  text(bubble, bx + 7, by + 27, d["map.alertSub"], "fm-bubble-sub", "start");
  const btnY = by + 42, btnH = 22;
  const ok = el("rect", { x: bx, y: btnY, width: bwid, height: btnH, rx: 6, class: "fm-btn fm-btn-ok" }, bubble);
  text(bubble, bx + bwid / 2, btnY + 15, "✓ " + d["map.approve"], "fm-btn-text fm-btn-ok-text");
  el("rect", { x: bx, y: btnY + btnH + 6, width: bwid, height: btnH, rx: 6, class: "fm-btn" }, bubble);
  text(bubble, bx + bwid / 2, btnY + btnH + 21, "✎ " + d["map.changes"], "fm-btn-text");
  if (!still) {
    const a = arrive.review, l = leave.review;
    animate(bubble, "opacity", [[a + 0.2, 0], [a + 0.5, 1], [l + 0.4, 1], [l + 0.9, 0]]);
    const tap = el("circle", { cx: bx + bwid * 0.7, cy: btnY + btnH / 2, r: 0, class: "fm-tap" }, bubble);
    animate(tap, "r", [[l - 1.0, 0], [l - 0.7, 14], [l - 0.4, 0]]);
    animate(tap, "opacity", [[l - 1.0, 0.9], [l - 0.4, 0]]);
    animate(linkPath, "opacity", [[a, 0.35], [a + 0.25, 1], [l, 1], [l + 0.4, 0.35]]);
  }

  // The task token that travels the rail.
  if (!still) {
    const total = pts.reduce((acc, p, i) => (i ? acc + Math.hypot(p[0] - pts[i - 1][0], p[1] - pts[i - 1][1]) : 0), 0);
    let run = 0;
    const frac = pts.map((p, i) => {
      if (i) run += Math.hypot(p[0] - pts[i - 1][0], p[1] - pts[i - 1][1]);
      return run / total;
    });
    const times = [], keys = [];
    STOPS.forEach((s, i) => {
      times.push(arrive[s], leave[s]);
      keys.push(frac[i], frac[i]);
    });
    times.unshift(0); keys.unshift(0);
    times.push(LOOP); keys.push(1);
    const token = el("g", { class: "fm-token" }, svg);
    el("circle", { r: 13, class: "fm-token-glow", filter: `url(#fm-glow-${layoutName})` }, token);
    el("circle", { r: 7, class: "fm-token-dot" }, token);
    el("animateMotion", {
      dur: `${LOOP}s`, repeatCount: "indefinite", calcMode: "linear",
      path: "M " + pts.map((p) => p.join(" ")).join(" L "),
      keyTimes: times.map(k).join(";"), keyPoints: keys.map((x) => x.toFixed(4)).join(";"),
    }, token);
    animate(token, "opacity", [[0, 0], [0.3, 1], [LOOP - 0.4, 1], [LOOP - 0.1, 0]]);
  }
  return svg;
}

export function renderFlowmap(container, dict) {
  if (!container) return;
  const still = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  container.replaceChildren(build("wide", dict, still), build("tall", dict, still));
}
