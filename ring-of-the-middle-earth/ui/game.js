/* Ring of the Middle Earth — command map UI.
 * Vanilla JS + SVG + SSE (no frameworks, per spec §37).
 * Talks to the Go engine: /config/map, /config/units, /game/state, /order,
 * /game/tick, /game/reset, /analysis/*, /events.
 */
"use strict";
const $ = (s, r = document) => r.querySelector(s);
const $$ = (s, r = document) => [...r.querySelectorAll(s)];
const SVGNS = "http://www.w3.org/2000/svg";

/* Geographic-ish layout for the 22 regions (NW Shire → SE Mordor). */
const LAYOUT = {
  "the-shire": [70, 90], "bree": [195, 115], "weathertop": [310, 95], "rivendell": [425, 80],
  "tharbad": [175, 235], "moria": [475, 195], "lothlorien": [525, 270], "fangorn": [430, 335],
  "fords-of-isen": [250, 345], "isengard": [345, 390], "rohan-plains": [475, 415],
  "helms-deep": [320, 480], "edoras": [435, 500], "emyn-muil": [625, 320], "dead-marshes": [705, 385],
  "minas-tirith": [560, 545], "ithilien": [725, 475], "osgiliath": [650, 565],
  "minas-morgul": [785, 565], "cirith-ungol": [875, 505], "mordor": [865, 650], "mount-doom": [950, 615],
};
const TERR = { PLAINS: "", MOUNTAINS: "⛰", FOREST: "🌲", FORTRESS: "🏰", VOLCANIC: "🌋", SWAMP: "🌫" };
const ICON = { RingBearer: "💍", FellowshipGuard: "⚔", GondorArmy: "🛡", Nazgul: "👁", UrukHaiLegion: "🪓", Maia: "🧙" };

/* Canonical routes (path-id lists) — spec §2.3. Used by the route quick-picks. */
const ROUTES = {
  route1: ["shire-to-bree", "bree-to-weathertop", "weathertop-to-rivendell", "rivendell-to-moria", "moria-to-lothlorien", "lothlorien-to-emyn-muil", "emyn-muil-to-ithilien", "ithilien-to-cirith-ungol", "cirith-ungol-to-mount-doom"],
  route2: ["shire-to-bree", "bree-to-rivendell", "rivendell-to-lothlorien", "lothlorien-to-emyn-muil", "emyn-muil-to-dead-marshes", "dead-marshes-to-ithilien", "ithilien-to-cirith-ungol", "cirith-ungol-to-mount-doom"],
  route3: ["shire-to-bree", "bree-to-rivendell", "rivendell-to-lothlorien", "lothlorien-to-emyn-muil", "emyn-muil-to-dead-marshes", "dead-marshes-to-mordor", "mordor-to-mount-doom"],
  route4: ["shire-to-tharbad", "tharbad-to-fords-of-isen", "fords-of-isen-to-edoras", "edoras-to-minas-tirith", "minas-tirith-to-osgiliath", "osgiliath-to-minas-morgul", "minas-morgul-to-cirith-ungol", "cirith-ungol-to-mount-doom"],
};

/* Demo scenarios — EXACT sequences verified against the live engine
 * (see verify-scenarios.ps1: 15/15 pass). */
const SCENARIOS = {
  s1: {
    name: "Information Hiding", steps: [
      { reset: true },
      { note: "Light routes the Ring Bearer toward Weathertop." },
      { order: { side: "light", orderType: "ASSIGN_ROUTE", unitId: "ring-bearer", pathIds: ["shire-to-bree", "bree-to-weathertop"] } },
      { note: "Dark deploys the Witch-King (detection range 2) to Bree." },
      { order: { side: "dark", orderType: "DEPLOY_NAZGUL", unitId: "witch-king", targetRegion: "bree" } },
      { tick: 4, note: "Advancing past the hidden start (turn 3)…" },
      { done: "Dark received RING_BEARER_DETECTED; the Light position stayed hidden from Dark (region = \"\")." },
    ]
  },
  s2a: {
    name: "Saruman corrupts a path", steps: [
      { reset: true },
      { note: "Saruman moves to the Fords of Isen." },
      { order: { side: "dark", orderType: "ASSIGN_ROUTE", unitId: "saruman", pathIds: ["fords-of-isen-to-isengard"] } },
      { tick: 1 },
      { note: "Same MAIA_ABILITY order type → Saruman CORRUPTS fords-of-isen-to-edoras." },
      { order: { side: "dark", orderType: "MAIA_ABILITY", unitId: "saruman", targetPathId: "fords-of-isen-to-edoras" } },
      { tick: 1 },
      { done: "fords-of-isen-to-edoras is permanently surveilled (level 3, corrupted) — purple on the map." },
    ]
  },
  s2b: {
    name: "Gandalf opens a blocked path", steps: [
      { reset: true },
      { note: "A Nazgûl deploys to Moria and blocks moria-to-lothlorien." },
      { order: { side: "dark", orderType: "DEPLOY_NAZGUL", unitId: "nazgul-2", targetRegion: "moria" } },
      { tick: 1 },
      { order: { side: "dark", orderType: "BLOCK_PATH", unitId: "nazgul-2", pathId: "moria-to-lothlorien" } },
      { tick: 1, note: "Path is now BLOCKED (red)." },
      { note: "Gandalf moves to Moria…" },
      { order: { side: "light", orderType: "ASSIGN_ROUTE", unitId: "gandalf", pathIds: ["rivendell-to-moria"] } },
      { tick: 1 },
      { note: "Same MAIA_ABILITY order type → Gandalf OPENS the path." },
      { order: { side: "light", orderType: "MAIA_ABILITY", unitId: "gandalf", targetPathId: "moria-to-lothlorien" } },
      { tick: 1 },
      { done: "moria-to-lothlorien is TEMPORARILY_OPEN (blue, 2 turns) — same order type, opposite effect." },
    ]
  },
  s2c: {
    name: "Guard denies a Nazgûl block", steps: [
      { reset: true },
      { note: "Legolas (a FellowshipGuard) takes the Lothlórien endpoint; a Nazgûl deploys opposite." },
      { order: { side: "light", orderType: "ASSIGN_ROUTE", unitId: "legolas", pathIds: ["rivendell-to-lothlorien"] } },
      { order: { side: "dark", orderType: "DEPLOY_NAZGUL", unitId: "nazgul-3", targetRegion: "emyn-muil" } },
      { tick: 1 },
      { note: "Nazgûl tries to block lothlorien-to-emyn-muil…" },
      { order: { side: "dark", orderType: "BLOCK_PATH", unitId: "nazgul-3", pathId: "lothlorien-to-emyn-muil" } },
      { tick: 1 },
      { done: "Block FAILED — the guard at the endpoint denies it; the path stays OPEN (spec §2.4)." },
    ]
  },
  win: {
    name: "Win Drive", steps: [
      { reset: true },
      { note: "Ring Bearer takes the Dark Route to Mount Doom." },
      { order: { side: "light", orderType: "ASSIGN_ROUTE", unitId: "ring-bearer", pathIds: ROUTES.route3 } },
      { tick: 7, note: "Advancing to Mount Doom…" },
      { note: "Destroy the Ring." },
      { order: { side: "light", orderType: "DESTROY_RING", unitId: "ring-bearer" } },
      { tick: 1 },
      { done: "Light Side wins — GameOver fires (exactly once on game.broadcast)." },
    ]
  },
};

/* ---------------- global app state ---------------- */
const App = {
  engineUrl: "",
  regions: {}, paths: {}, adj: {}, unitCfg: {}, hiddenUntilTurn: 3, maxTurns: 40,
  boards: {}, lastTurn: 0, routeRegions: [], busy: false,
};

async function api(method, path, body) {
  const opt = { method, headers: {} };
  if (body !== undefined) { opt.headers["Content-Type"] = "application/json"; opt.body = JSON.stringify(body); }
  const r = await fetch(App.engineUrl + path, opt);
  const txt = await r.text();
  let data = null; try { data = txt ? JSON.parse(txt) : null; } catch { data = txt; }
  return { ok: r.ok, status: r.status, data };
}
const getTurn = async () => (await api("GET", "/game/state?side=light")).data?.turn ?? 1;

/* ---------------- Board ---------------- */
class Board {
  constructor(side, root) {
    this.side = side; this.root = root; this.snap = null; this.es = null; this._t = null;
    root.querySelector(".side-badge").textContent = side === "dark" ? "DARK · The Shadow" : "LIGHT · Free Peoples";
    root.querySelector(".side-badge").classList.add(side);
    this.mapEl = root.querySelector(".board-map");
    this.logEl = root.querySelector(".board-log");
    this.rbEl = root.querySelector(".board-rb");
    this.connEl = root.querySelector(".board-conn");
    this.anEl = root.querySelector(".board-analysis");
    root.querySelector(".board-analyze").onclick = () => this.analyze();
    this.mapEl.addEventListener("click", (e) => {
      const g = e.target.closest("[data-region]"); if (g) onRegionClick(g.getAttribute("data-region"));
    });
  }
  connect() {
    if (this.es) this.es.close();
    this.es = new EventSource(`${App.engineUrl}/events?side=${this.side}`);
    this.es.onopen = () => { this.connEl.textContent = "● live"; this.connEl.style.color = "var(--free)"; };
    this.es.onerror = () => { this.connEl.textContent = "○ reconnecting"; this.connEl.style.color = "var(--shadow)"; };
    this.es.onmessage = (ev) => this.onEvent(ev.data);
    this.refresh();
  }
  onEvent(raw) {
    let d; try { d = JSON.parse(raw); } catch { return; }
    const t = d.type || (d.snapshot ? "WorldStateSnapshot" : "event");
    if (t === "WorldStateSnapshot") { this.debouncedRefresh(); return; }
    if (t === "RingBearerDetected") this.log(`⚠ RING_BEARER_DETECTED → ${d.regionId} (t${d.turn})`, "alert");
    else if (t === "RingBearerSpotted") this.log(`⚠ RING_BEARER_SPOTTED on ${d.pathId}`, "alert");
    else if (t === "RingBearerMoved") this.log(`💍 Ring Bearer → ${d.trueRegion} (LIGHT only)`, "ring");
    else if (t === "PathCorrupted") this.log(`🟣 PathCorrupted: ${d.pathId}`, "alert");
    else if (t === "BlockFailed") this.log(`🛡 Block denied on ${d.pathId} (${d.reason})`, "note");
    else if (t === "BattleResolved") this.log(`⚔ Battle @ ${d.regionId}: ${d.attackerWon ? "attacker won" : "held"}`, "combat");
    else if (t === "GameOver") this.log(`🏁 GAME OVER — ${d.winner} (${d.cause})`, "over");
    else if (t === "RouteBlocked") this.log(`⛔ ${d.unitId} blocked on ${d.pathId}`, "note");
    this.debouncedRefresh();
  }
  debouncedRefresh() { clearTimeout(this._t); this._t = setTimeout(() => this.refresh(), 180); }
  async refresh() {
    const r = await api("GET", `/game/state?side=${this.side}`);
    if (!r.ok) { this.connEl.textContent = "state error"; return; }
    this.snap = r.data; this.render();
    updateHud(this.snap); // turn/over present on both sides' snapshots
  }
  render() {
    const s = this.snap; if (!s) return;
    this.rbEl.textContent = this.side === "dark"
      ? (s.ringLastDetectedRegion ? `Last seen: ${name(s.ringLastDetectedRegion)} (t${s.ringLastDetectedTurn})` : "Ring Bearer: unknown")
      : `Ring Bearer: ${name(s.ringBearerRegion) || "?"}`;
    this.mapEl.innerHTML = renderSVG(s, this.side);
  }
  async analyze() {
    const ep = this.side === "dark" ? "/analysis/intercept" : "/analysis/routes";
    const r = await api("GET", ep);
    this.anEl.innerHTML = this.side === "dark" ? renderIntercept(r.data) : renderRoutes(r.data);
  }
  log(msg, cls) {
    const d = document.createElement("div"); d.className = "entry " + (cls || "");
    d.textContent = msg; this.logEl.prepend(d);
    while (this.logEl.children.length > 60) this.logEl.lastChild.remove();
  }
}

/* ---------------- SVG rendering ---------------- */
function xy(id) { return LAYOUT[id] || [500, 360]; }
function name(id) { return App.regions[id]?.name || id || ""; }

function renderSVG(snap, side) {
  let lines = "", surv = "", nodes = "", tokens = "";
  // paths
  for (const p of Object.values(App.paths)) {
    const a = xy(p.from), b = xy(p.to);
    const st = snap.paths?.[p.id] || { status: "OPEN", surveillanceLevel: 0 };
    let cls = "pathline";
    if (st.status === "THREATENED") cls += " threat";
    else if (st.status === "BLOCKED") cls += " blocked";
    else if (st.status === "TEMPORARILY_OPEN") cls += " temp";
    lines += `<line class="${cls}" x1="${a[0]}" y1="${a[1]}" x2="${b[0]}" y2="${b[1]}"/>`;
    if ((st.surveillanceLevel || 0) > 0 || st.corrupted)
      surv += `<line class="pathsurv" x1="${a[0]}" y1="${a[1]}" x2="${b[0]}" y2="${b[1]}"/>`;
  }
  // Ring Bearer marker.
  //  LIGHT sees the REAL location (pulsing gold ring + 💍 token).
  //  DARK sees ONLY the last region detection revealed (red ✦), and nothing at all
  //  before the first detection — never the live position.
  if (side === "light" && snap.ringBearerRegion) {
    const c = xy(snap.ringBearerRegion);
    nodes += `<circle class="ring-marker" cx="${c[0]}" cy="${c[1]}" r="30"/>`;
    tokens += `<text class="rbtoken" x="${c[0]}" y="${c[1] - 22}">💍</text>`;
  }
  if (side === "dark" && snap.ringLastDetectedRegion) {
    const c = xy(snap.ringLastDetectedRegion);
    nodes += `<circle class="detect-marker" cx="${c[0]}" cy="${c[1]}" r="30"/>`;
    tokens += `<text class="rbtoken detect" x="${c[0]}" y="${c[1] - 22}">✦</text>`;
  }
  // regions
  for (const r of Object.values(App.regions)) {
    const [x, y] = xy(r.id); const rs = snap.regions?.[r.id] || {};
    const ctrl = (rs.controlledBy || r.startControl || "NEUTRAL").toLowerCase().includes("free") ? "free"
      : (rs.controlledBy || r.startControl).toLowerCase().includes("shadow") ? "shadow" : "neutral";
    const sel = App.selRegion === r.id ? " sel" : "";
    const fort = rs.fortified ? " fort" : "";
    nodes += `<g class="region ${ctrl}${sel}${fort}" data-region="${r.id}">
      <circle class="rnode" cx="${x}" cy="${y}" r="20"/>
      <text class="rterr" x="${x}" y="${y + 5}">${TERR[r.terrain] || ""}</text>
      <text class="rname" x="${x}" y="${y + 35}">${r.name}</text></g>`;
  }
  // unit tokens grouped per region
  const byRegion = {};
  for (const u of Object.values(snap.units || {})) {
    if (!u.region || u.status === "DESTROYED" || u.status === "RESPAWNING") continue;
    (byRegion[u.region] ||= []).push(u);
  }
  for (const [rid, us] of Object.entries(byRegion)) {
    const [x, y] = xy(rid);
    us.forEach((u, i) => {
      const cfg = App.unitCfg[u.id] || {}; const ic = ICON[cfg.class] || "•";
      const col = cfg.side === "SHADOW" ? "var(--shadow)" : "var(--free)";
      const ang = -Math.PI / 2 + (i - (us.length - 1) / 2) * 0.7; const rr = 30;
      const tx = x + rr * Math.cos(ang), ty = y - 18 + rr * Math.sin(ang);
      tokens += `<text class="utoken" x="${tx}" y="${ty}" style="fill:${col}">${ic}</text>`
        + `<text class="ubadge" x="${tx}" y="${ty + 9}">${u.strength}</text>`;
    });
  }
  return `<svg viewBox="0 0 1000 720" preserveAspectRatio="xMidYMid meet">${lines}${surv}${nodes}${tokens}</svg>`;
}

function renderRoutes(d) {
  if (!d || !d.routes) return "<i>no data</i>";
  const max = Math.max(1, ...d.routes.map(r => r.riskScore));
  let h = `<b>Route risk</b> ${d.partial ? "<i>(partial)</i>" : ""}`;
  for (const r of d.routes) {
    const rec = r.routeId === d.recommended ? "an-rec" : "";
    h += `<div class="an-route"><span class="${rec}" style="width:120px">${r.routeId.replace("route-", "").slice(0, 14)}</span>
      <div class="an-bar" style="width:${10 + 90 * r.riskScore / max}px"></div><span>${r.riskScore}</span></div>`;
  }
  h += `<div style="margin-top:6px">✓ recommended: <span class="an-rec">${(d.recommended || "").replace("route-", "")}</span></div>`;
  return h;
}
function renderIntercept(d) {
  if (!d || !d.byUnit) return "<i>no data</i>";
  let h = `<b>Interception plan</b> ${d.partial ? "<i>(partial)</i>" : ""}`;
  if (!d.byUnit.length) return h + "<div><i>no Nazgûl candidates</i></div>";
  for (const e of d.byUnit)
    h += `<div class="an-route"><span style="width:90px">${e.unitId}</span><span style="width:90px">${name(e.targetRegion)}</span><span>${e.score.toFixed(2)}</span></div>`;
  return h;
}

/* ---------------- HUD ---------------- */
function updateHud(snap) {
  App.lastTurn = snap.turn;
  $("#hudTurn").textContent = "Turn " + snap.turn;
  $("#hudPhase").textContent = snap.turn <= App.hiddenUntilTurn ? "hidden start (no detection)" : "detection active";
  const go = $("#gameover");
  if (snap.over) {
    go.classList.remove("hidden");
    go.textContent = snap.winner === "NONE" ? `DRAW (turn ${snap.turn})` : `🏁 ${snap.winner} WIN — ${snap.cause}`;
  } else go.classList.add("hidden");
}

/* ---------------- order builder ---------------- */
const LEGAL = {
  RingBearer: ["ASSIGN_ROUTE", "REDIRECT_UNIT", "DESTROY_RING"],
  FellowshipGuard: ["ASSIGN_ROUTE", "REDIRECT_UNIT", "BLOCK_PATH", "ATTACK_REGION", "REINFORCE_REGION"],
  GondorArmy: ["ASSIGN_ROUTE", "REDIRECT_UNIT", "FORTIFY_REGION", "ATTACK_REGION", "REINFORCE_REGION"],
  Maia: ["ASSIGN_ROUTE", "REDIRECT_UNIT", "MAIA_ABILITY"],
  Nazgul: ["ASSIGN_ROUTE", "REDIRECT_UNIT", "DEPLOY_NAZGUL", "BLOCK_PATH", "SEARCH_PATH", "ATTACK_REGION"],
  UrukHaiLegion: ["ASSIGN_ROUTE", "REDIRECT_UNIT", "BLOCK_PATH", "ATTACK_REGION", "REINFORCE_REGION"],
};
function populateUnits() {
  const side = $("#ordSide").value;
  const wantSide = side === "dark" ? "SHADOW" : "FREE_PEOPLES";
  const sel = $("#ordUnit"); sel.innerHTML = "";
  for (const [id, c] of Object.entries(App.unitCfg))
    if (c.side === wantSide) sel.add(new Option(`${c.name} (${id})`, id));
  populateOrderTypes();
}
function populateOrderTypes() {
  const id = $("#ordUnit").value; const c = App.unitCfg[id] || {};
  const sel = $("#ordType"); sel.innerHTML = "";
  (LEGAL[c.class] || ["ASSIGN_ROUTE"]).forEach(t => sel.add(new Option(t, t)));
  renderOrderInputs();
}
function renderOrderInputs() {
  const t = $("#ordType").value; const box = $("#ordInputs"); box.innerHTML = "";
  $("#routeBuild").classList.add("hidden");
  const pathOpts = () => Object.keys(App.paths).sort().map(p => `<option>${p}</option>`).join("");
  const regOpts = () => Object.keys(App.regions).sort().map(r => `<option>${r}</option>`).join("");
  if (t === "ASSIGN_ROUTE" || t === "REDIRECT_UNIT") {
    $("#routeBuild").classList.remove("hidden"); seedRoute();
  } else if (t === "BLOCK_PATH" || t === "SEARCH_PATH") {
    box.innerHTML = `<label class="fld">Path<select id="fPath">${pathOpts()}</select></label>`;
  } else if (t === "MAIA_ABILITY") {
    box.innerHTML = `<label class="fld">Target path<select id="fPath">${pathOpts()}</select></label>`;
  } else if (t === "ATTACK_REGION" || t === "DEPLOY_NAZGUL" || t === "REINFORCE_REGION") {
    box.innerHTML = `<label class="fld">Target region<select id="fRegion">${regOpts()}</select></label>`;
  } else if (t === "FORTIFY_REGION" || t === "DESTROY_RING") {
    box.innerHTML = `<div class="hint">No extra input.</div>`;
  }
}
function currentRegionOf(id) {
  const lb = App.boards.light; if (!lb || !lb.snap) return null;
  if (App.unitCfg[id]?.class === "RingBearer") return lb.snap.ringBearerRegion || null;
  return lb.snap.units?.[id]?.region || null;
}
function seedRoute() {
  const id = $("#ordUnit").value; const start = currentRegionOf(id);
  App.routeRegions = start ? [start] : []; renderRoute();
}
function renderRoute() {
  $("#rbList").textContent = App.routeRegions.map(name).join("  →  ") || "(click a region to start)";
  const pids = routeToPaths(App.routeRegions);
  $("#rbPaths").textContent = pids.ok ? pids.paths.join(", ") : "⚠ " + pids.error;
}
function routeToPaths(regs) {
  const paths = [];
  for (let i = 0; i + 1 < regs.length; i++) {
    const pid = App.adj[regs[i] + "|" + regs[i + 1]];
    if (!pid) return { ok: false, error: `no path ${regs[i]}→${regs[i + 1]}`, paths };
    paths.push(pid);
  }
  return { ok: true, paths };
}
function onRegionClick(rid) {
  App.selRegion = rid;
  const t = $("#ordType")?.value;
  if (($("#tab-order").classList.contains("active")) && (t === "ASSIGN_ROUTE" || t === "REDIRECT_UNIT")) {
    if (App.routeRegions[App.routeRegions.length - 1] !== rid) App.routeRegions.push(rid);
    renderRoute();
  }
  for (const b of Object.values(App.boards)) b.render();
}

async function submitOrder() {
  const side = $("#ordSide").value, unitId = $("#ordUnit").value, orderType = $("#ordType").value;
  const o = { side, orderType, unitId };
  if (orderType === "ASSIGN_ROUTE" || orderType === "REDIRECT_UNIT") {
    const r = routeToPaths(App.routeRegions);
    if (!r.ok) return orderResult(false, r.error);
    if (orderType === "ASSIGN_ROUTE") o.pathIds = r.paths; else o.newPathIds = r.paths;
  } else if ($("#fPath")) { o[orderType === "MAIA_ABILITY" ? "targetPathId" : "pathId"] = $("#fPath").value; }
  else if ($("#fRegion")) o.targetRegion = $("#fRegion").value;
  const res = await sendOrder(o);
  orderResult(res.ok, res.ok ? "ACCEPTED" : (res.data?.errorCode || res.status));
  refreshAll();
}
function orderResult(ok, msg) {
  const el = $("#ordResult"); el.className = "ord-result " + (ok ? "ok" : "bad");
  el.textContent = (ok ? "✓ " : "✗ ") + msg;
}
async function sendOrder(o) {
  const turn = await getTurn();
  const body = { orderType: o.orderType, playerId: o.side, unitId: o.unitId, turn };
  for (const k of ["pathIds", "newPathIds", "targetPathId", "pathId", "targetRegion"]) if (o[k] !== undefined) body[k] = o[k];
  return api("POST", "/order", body);
}

/* ---------------- global controls ---------------- */
async function tick() { await api("POST", "/game/tick"); await refreshAll(); }
async function reset() { await api("POST", "/game/reset"); App.routeRegions = []; renderRoute?.(); await refreshAll(); }
async function refreshAll() { for (const b of Object.values(App.boards)) await b.refresh(); }

/* ---------------- scenario runner ---------------- */
const sleep = (ms) => new Promise(r => setTimeout(r, ms));
async function runScenario(key) {
  if (App.busy) return; App.busy = true;
  $$(".scn").forEach(b => b.disabled = true);
  const sc = SCENARIOS[key]; const status = $("#scnStatus");
  for (const b of Object.values(App.boards)) b.log(`▶ Scenario: ${sc.name}`, "note");
  try {
    for (const step of sc.steps) {
      if (step.note) { status.textContent = step.note; for (const b of Object.values(App.boards)) b.log("• " + step.note, "note"); }
      if (step.reset) { await api("POST", "/game/reset"); await refreshAll(); }
      if (step.order) {
        const res = await sendOrder(step.order);
        const tag = `${step.order.orderType} ${step.order.unitId}`;
        for (const b of Object.values(App.boards)) b.log((res.ok ? "✓ " : "✗ ") + tag + (res.ok ? "" : " — " + (res.data?.errorCode || res.status)), res.ok ? "" : "alert");
        await refreshAll();
      }
      if (step.tick) for (let i = 0; i < step.tick; i++) { await api("POST", "/game/tick"); await refreshAll(); await sleep(550); }
      if (step.done) { status.textContent = "✓ " + step.done; for (const b of Object.values(App.boards)) b.log("✓ " + step.done, "over"); }
      await sleep(450);
    }
    if (App.boards.dark) App.boards.dark.analyze();
    if (App.boards.light) App.boards.light.analyze();
  } finally { App.busy = false; $$(".scn").forEach(b => b.disabled = false); }
}

/* ---------------- init ---------------- */
function buildBoards() {
  const cont = $("#boards"); cont.innerHTML = ""; App.boards = {};
  const mode = $("#viewMode").value;
  const sides = mode === "light" ? ["light"] : mode === "dark" ? ["dark"] : ["light", "dark"];
  for (const side of sides) {
    const node = $("#boardTpl").content.firstElementChild.cloneNode(true);
    cont.appendChild(node);
    App.boards[side] = new Board(side, node);
  }
}
async function connect() {
  App.engineUrl = $("#engineUrl").value.replace(/\/$/, "");
  // load config (map + units)
  const m = await api("GET", "/config/map"); const u = await api("GET", "/config/units");
  if (!m.ok || !u.ok) { $("#hudConn").textContent = "engine unreachable"; $("#hudConn").className = "hudpill off"; return; }
  App.regions = {}; App.paths = {}; App.adj = {}; App.unitCfg = {};
  for (const r of m.data.regions) App.regions[r.id] = r;
  for (const p of m.data.paths) { App.paths[p.id] = p; App.adj[p.from + "|" + p.to] = p.id; App.adj[p.to + "|" + p.from] = p.id; }
  for (const c of u.data.units) App.unitCfg[c.id] = c;
  App.hiddenUntilTurn = u.data.hiddenUntilTurn ?? 3; App.maxTurns = u.data.maxTurns ?? 40;
  $("#hudConn").textContent = "connected"; $("#hudConn").className = "hudpill on";
  buildBoards();
  for (const b of Object.values(App.boards)) b.connect();
  populateUnits();
}

function initUI() {
  $("#engineUrl").value = (location.protocol.startsWith("http") ? location.origin : "http://localhost:8080");
  $("#btnConnect").onclick = connect;
  $("#btnTick").onclick = tick;
  $("#btnReset").onclick = reset;
  $("#viewMode").onchange = () => { buildBoards(); for (const b of Object.values(App.boards)) b.connect(); };
  $("#ordSide").onchange = populateUnits;
  $("#ordUnit").onchange = populateOrderTypes;
  $("#ordType").onchange = renderOrderInputs;
  $("#ordSubmit").onclick = submitOrder;
  $("#rbClear").onclick = () => { seedRoute(); };
  $$(".rb-quick .mini").forEach(b => b.onclick = () => {
    const pids = ROUTES[b.dataset.route]; const start = pids.length ? App.paths[pids[0]].from : null;
    // rebuild region list from path chain
    let cur = start; App.routeRegions = start ? [start] : [];
    for (const pid of pids) { const p = App.paths[pid]; cur = (cur === p.from) ? p.to : p.from; App.routeRegions.push(cur); }
    renderRoute();
  });
  $$(".tab").forEach(t => t.onclick = () => {
    $$(".tab").forEach(x => x.classList.remove("active")); t.classList.add("active");
    $$(".tabpane").forEach(p => p.classList.remove("active")); $("#tab-" + t.dataset.tab).classList.add("active");
  });
  $$(".scn").forEach(b => b.onclick = () => runScenario(b.dataset.scn));
  connect();
}
document.addEventListener("DOMContentLoaded", initUI);
