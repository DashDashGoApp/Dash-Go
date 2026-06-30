#!/usr/bin/env node
// Hermetic Lite-radar fixture: no network, browser, X11, or generated bundle.
// It verifies complete tile coverage, newest-first progressive paint, bounded
// parallel fetches, zoom/session caches, and full close-time teardown.
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, "..");
const sources = fs.readFileSync(path.join(root, "ui/js/radar-sources.js"), "utf8");
const overlay = fs.readFileSync(path.join(root, "ui/js/radar-overlay.js"), "utf8");
const lite = fs.readFileSync(path.join(root, "ui/js/radar-lite.js"), "utf8");
const fetches = [];
const images = [];
const bitmaps = [];
const active = { base: 0, radar: 0 };
const peak = { base: 0, radar: 0 };
let madeCanvases = 0;
let slowFollowupFrames = true;
let slowAllTiles = false;
let failNextImage = null;

function classList() {
  const values = new Set();
  return {
    add: (...names) => names.forEach(name => values.add(name)),
    remove: (...names) => names.forEach(name => values.delete(name)),
    contains: name => values.has(name),
    toggle: (name, force) => {
      const on = force === undefined ? !values.has(name) : !!force;
      if (on) values.add(name); else values.delete(name);
      return on;
    },
  };
}

function element(id = "") {
  let html = "";
  return {
    id,
    hidden: false,
    disabled: false,
    textContent: "",
    style: {},
    dataset: {},
    classList: classList(),
    children: [],
    attributes: {},
    setAttribute(name, value) { this.attributes[name] = String(value); },
    addEventListener() {},
    appendChild(child) { this.children.push(child); return child; },
    querySelectorAll() { return []; },
    get innerHTML() { return html; },
    set innerHTML(value) { html = String(value); this.children = []; },
  };
}

function canvas(id = "") {
  const node = element(id);
  node.width = 1;
  node.height = 1;
  node.ops = [];
  node.contextOptions = [];
  node.getContext = (_kind, options) => {
    node.contextOptions.push(options || {});
    return {
      clearRect: (...args) => node.ops.push(["clearRect", ...args]),
      fillRect: (...args) => node.ops.push(["fillRect", ...args]),
      drawImage: (...args) => node.ops.push(["drawImage", ...args]),
      set fillStyle(value) { node.ops.push(["fillStyle", value]); },
    };
  };
  return node;
}

const nodes = new Map();
for (const id of [
  "radarfull", "radarbase", "radarframes", "radarlitecanvas", "radarstage",
  "radarbusy", "radarerror", "radarattribution", "radartitle", "radarprovider",
  "radarstamp", "radarprev", "radarplay", "radarnext", "radarnow", "radarrefresh",
  "radarzoom", "radarscrub", "radarclose",
]) nodes.set(id, id === "radarlitecanvas" ? canvas(id) : element(id));
nodes.get("radarstage").clientWidth = 1024;
nodes.get("radarstage").clientHeight = 600;

function imageKind(url) {
  try { return new URL(String(url)).hostname === "tile.openstreetmap.org" ? "base" : "radar"; }
  catch (_) { return "radar"; }
}
function imageDelay(url) {
  const value = String(url);
  if (slowAllTiles) return 60;
  return slowFollowupFrames && (value.includes("/older") || value.includes("/recent")) ? 40 : 0;
}

class FakeImage {
  constructor() {
    this.complete = false;
    this.naturalWidth = 0;
    this.onload = null;
    this.onerror = null;
    this.decoding = "";
    this._src = "";
    this._timer = null;
    this._kind = "";
  }
  get src() { return this._src; }
  set src(value) {
    if (this._timer) {
      clearTimeout(this._timer);
      this._timer = null;
      if (this._kind) active[this._kind]--;
      this._kind = "";
    }
    this._src = String(value || "");
    if (!this._src) {
      this.complete = false;
      this.naturalWidth = 0;
      return;
    }
    images.push(this._src);
    this._kind = imageKind(this._src);
    active[this._kind]++;
    peak[this._kind] = Math.max(peak[this._kind], active[this._kind]);
    this._timer = setTimeout(() => {
      this._timer = null;
      const kind = this._kind;
      this._kind = "";
      if (kind) active[kind]--;
      if (!this._src) return;
      if (failNextImage && failNextImage(this._src)) {
        failNextImage = null;
        this.complete = true;
        this.naturalWidth = 0;
        this.onerror?.();
        return;
      }
      this.complete = true;
      this.naturalWidth = 256;
      this.onload?.();
    }, imageDelay(this._src));
  }
}

const delayed = new Set();
const shortTimeout = (fn, ms, ...args) => {
  if (Number(ms) >= 1000) {
    const token = { delayed: true };
    delayed.add(token);
    return token;
  }
  return setTimeout(fn, ms, ...args);
};
const shortClearTimeout = token => {
  if (token && token.delayed) delayed.delete(token);
  else clearTimeout(token);
};
const delay = ms => new Promise(resolve => setTimeout(resolve, ms));
async function waitFor(predicate, message) {
  const deadline = Date.now() + 1500;
  while (!predicate()) {
    if (Date.now() >= deadline) throw new Error(`timed out: ${message}`);
    await delay(2);
  }
}
function countImages(fragment) { return images.filter(value => value.includes(fragment)).length; }

const context = vm.createContext({
  console,
  document: {
    addEventListener() {},
    getElementById: id => nodes.get(id) || null,
    createElement: tag => {
      if (tag === "canvas") {
        madeCanvases++;
        return canvas("scratch");
      }
      return element(tag);
    },
  },
  Image: FakeImage,
  CONFIG: {
    profile: "lite",
    lat: 41.8781,
    lon: -87.6298,
  },
  setTimeout: shortTimeout,
  clearTimeout: shortClearTimeout,
  setInterval,
  clearInterval,
  performance: { now: () => Date.now() },
  requestAnimationFrame: callback => setTimeout(callback, 0),
  window: { addEventListener() {} },
  createImageBitmap: async source => {
    const bitmap = { width: source.width, height: source.height, closed: false, close() { this.closed = true; } };
    bitmaps.push(bitmap);
    return bitmap;
  },
  pauseUiAnimations() {},
  resumeUiAfterOverlay() {},
  disarmOverlayAutoClose() {},
  armOverlayAutoClose() {},
  overlayIsOpen: () => false,
  bindTap() {},
  fetch: async url => {
    const value = String(url);
    fetches.push(value);
    if (value === "/api/radar/status") {
      return { ok: true, json: async () => ({ provider: "rainviewer", providers: [] }) };
    }
    if (value === "https://api.rainviewer.com/public/weather-maps.json") {
      return {
        ok: true,
        json: async () => ({
          host: "https://radar.example",
          radar: {
            past: [
              { path: "/oldest", time: 10 },
              { path: "/older", time: 20 },
              { path: "/recent", time: 30 },
              { path: "/latest", time: 40 },
            ],
          },
        }),
      };
    }
    throw new Error(`unexpected fetch ${value}`);
  },
});

vm.runInContext(
  `${sources}\n${overlay}\n${lite}\nglobalThis.__radarTest={openRadar,closeRadar,radarRefreshLite,radarLiteToggleZoom,radarLiteStep,radarLiteNow,radarLiteSetPlaying,radarLitePx,radarLiteTilePlan,radarLiteFrameLimitForZoom,RADAR_STATE,radarFrameCount};`,
  context,
  { filename: "lite-radar-source.js" },
);
const radar = context.__radarTest;

assert.equal(radar.radarFrameCount("rainviewer"), 1, "RainViewer counts source frames after its timeline has loaded");
assert.equal(radar.radarFrameCount("nws"), 1, "non-animated Lite providers retain a single current frame");

function planCoversViewport(plan, px) {
  const probes=[[0,0],[px-1,0],[0,px-1],[px-1,px-1],[px/2,0],[px/2,px-1],[0,px/2],[px-1,px/2]];
  for (const [x,y] of probes) {
    const covered=plan.slots.some(slot=>{
      const left=plan.ox+slot.col*256,top=plan.oy+slot.row*256;
      return x>=left && x<left+256 && y>=top && y<top+256;
    });
    assert.equal(covered,true,`tile plan covers viewport edge probe ${x},${y}`);
  }
}
for (const px of [384,512,640,768]) {
  for (const [lat,lon] of [[41.8781,-87.6298],[0,0],[84.9,179.99],[-84.9,-179.99]]) {
    for (const zoom of [6,7]) {
      const plan=radar.radarLiteTilePlan(px,lat,lon,zoom);
      assert.equal(plan.planned,plan.slots.length,"planned coverage count matches slot plan");
      planCoversViewport(plan,px);
    }
  }
}

const opening = radar.openRadar();
await waitFor(() => !nodes.get("radarlitecanvas").hidden && radar.RADAR_STATE.liteFrames.filter(Boolean).length === 1, "newest Lite frame to paint before older animation frames");
assert.equal(radar.RADAR_STATE.liteFrameIdx, 3, "first visible Lite frame is the newest one");
assert.equal(nodes.get("radarbusy").hidden, true, "spinner clears after the first usable frame, not after all animation frames");
assert.equal(nodes.get("radarplay").disabled, true, "Play remains disabled until a second completed frame exists");
assert.equal(nodes.get("radarlitecanvas").width, 600, "visible canvas uses the measured stage short dimension");
assert.equal(nodes.get("radarlitecanvas").height, 600, "visible canvas remains square");
assert.ok(nodes.get("radarlitecanvas").contextOptions.some(options => options.alpha === false && options.desynchronized === true), "visible Lite canvas uses opaque low-latency compositing");
await opening;

assert.equal(radar.radarLiteFrameLimitForZoom(7), 4, "Lite local zoom retains every returned source frame");
assert.equal(fetches.filter(value => value === "https://api.rainviewer.com/public/weather-maps.json").length, 1, "one RainViewer frame-index request on initial open");
assert.equal(countImages("tile.openstreetmap.org"), 9, "Lite derives a fully covered Chicago OSM viewport plan for this regional center");
assert.equal(countImages("radar.example/older"), 9, "older RainViewer frame follows the complete Lite tile plan");
assert.equal(countImages("radar.example/recent"), 9, "recent RainViewer frame follows the complete Lite tile plan");
assert.equal(countImages("radar.example/latest"), 9, "newest RainViewer frame follows the complete Lite tile plan");
assert.equal(countImages("radar.example/oldest"), 9, "the complete source timeline includes the oldest frame");
assert.equal(images.length, 45, "initial Lite view fetches one fully covered base plus every source radar frame");
assert.equal(peak.base, 2, "OSM base loading never exceeds the two-request policy");
assert.equal(peak.radar, 3, "RainViewer loading uses the bounded three-request Lite pool");
assert.equal(nodes.get("radarlitecanvas").hidden, false, "snapshot remains visible after background frame completion");
assert.equal(nodes.get("radarframes").children.length, 0, "Lite must not build DOM frame layers");
assert.equal(nodes.get("radarbase").children.length, 0, "Lite base is only inside the snapshot canvas");
assert.equal(radar.RADAR_STATE.prefetchTimer, null, "Lite must not schedule prefetch");
assert.equal(madeCanvases, 2, "first render allocates one cached base and one pooled scratch canvas");
assert.deepEqual(Object.keys(radar.RADAR_STATE.liteBaseCache), ["6"], "regional base is cached for the open overlay session");
assert.equal(bitmaps.length, 4, "one radar-only bitmap per source Lite frame");
for (const id of ["radarprev", "radarplay", "radarnext", "radarnow", "radarscrub", "radarzoom", "radarrefresh"]) {
  assert.equal(nodes.get(id).hidden, false, `${id} is visible for Lite`);
}
assert.equal(nodes.get("radarplay").disabled, false, "Play is enabled with multiple Lite frames");
assert.equal(nodes.get("radarscrub").disabled, false, "scrub is enabled with multiple Lite frames");
assert.equal(nodes.get("radarzoom").textContent, "Zoom in", "Lite begins at regional zoom");
assert.equal(radar.RADAR_STATE.liteFrameIdx, 3, "Lite begins on the newest frame");

radar.radarLiteStep(-1);
assert.equal(radar.RADAR_STATE.liteFrameIdx, 2, "previous control steps a local composite");
assert.match(nodes.get("radarstamp").textContent, /3\/4$/, "frame label includes the local frame counter");
radar.radarLiteNow();
assert.equal(radar.RADAR_STATE.liteFrameIdx, 3, "Now returns to the newest local composite");
radar.radarLiteSetPlaying(true);
assert.equal(radar.RADAR_STATE.litePlaying, true, "Play starts bounded local playback");
radar.radarLiteSetPlaying(false);
assert.equal(radar.RADAR_STATE.litePlaying, false, "Pause stops the Lite timer");

const beforeRefresh = images.length;
const baseBeforeRefresh = countImages("tile.openstreetmap.org");
const jsonBeforeRefresh = fetches.filter(value => value === "https://api.rainviewer.com/public/weather-maps.json").length;
slowFollowupFrames = false;
await radar.radarRefreshLite();
assert.equal(images.length, beforeRefresh + 36, "refresh reuses the regional base and fetches every source radar frame");
assert.equal(countImages("tile.openstreetmap.org"), baseBeforeRefresh, "refresh does not re-fetch a cached regional base");
assert.equal(fetches.filter(value => value === "https://api.rainviewer.com/public/weather-maps.json").length, jsonBeforeRefresh + 1, "explicit refresh deliberately refreshes the RainViewer frame list");
assert.equal(madeCanvases, 2, "refresh reuses the cached base and pooled scratch canvases");
await radar.radarRefreshLite();
assert.equal(images.length, beforeRefresh + 36, "cooldown blocks a second immediate refresh");

radar.radarLiteSetPlaying(true);
const oldBitmaps = radar.RADAR_STATE.liteFrames.slice();
const beforeZoom = images.length;
const jsonBeforeZoom = fetches.filter(value => value === "https://api.rainviewer.com/public/weather-maps.json").length;
const zoomBuild = radar.radarLiteToggleZoom();
assert.equal(radar.RADAR_STATE.litePlaying, false, "zoom stops Lite playback before rebuilding");
assert.equal(radar.RADAR_STATE.liteTimer, null, "zoom clears the Lite playback timer before tile work");
assert.equal(nodes.get("radarlitecanvas").hidden, false, "zoom retains the old complete composite while the replacement is loading");
assert.ok(oldBitmaps.every(bitmap => !bitmap.closed), "zoom retains old frame bitmaps until the replacement is complete");
await zoomBuild;
assert.equal(radar.RADAR_STATE.liteZoom, 7, "Zoom in selects local zoom seven");
assert.equal(nodes.get("radarzoom").textContent, "Zoom out", "zoom control exposes the return action");
assert.equal(images.length, beforeZoom + 60, "local zoom builds one 4×3 base and every source radar frame");
assert.equal(images.slice(beforeZoom).filter(value => value.includes("/256/7/")).length, 48, "local zoom requests every complete local-zoom source frame");
assert.equal(fetches.filter(value => value === "https://api.rainviewer.com/public/weather-maps.json").length, jsonBeforeZoom, "zoom reuses the recent RainViewer frame list");
assert.ok(oldBitmaps.every(bitmap => bitmap.closed), "old frame bitmaps are released after the first local replacement commits");
assert.deepEqual(Object.keys(radar.RADAR_STATE.liteBaseCache).sort(), ["6", "7"], "both bounded zoom bases are cached while radar stays open");

const beforeZoomBack = images.length;
const baseBeforeZoomBack = countImages("tile.openstreetmap.org");
const jsonBeforeZoomBack = fetches.filter(value => value === "https://api.rainviewer.com/public/weather-maps.json").length;
await radar.radarLiteToggleZoom();
assert.equal(radar.RADAR_STATE.liteZoom, 6, "Zoom out returns to the regional view");
assert.equal(images.length, beforeZoomBack + 36, "zoom-back reuses the cached regional base and rebuilds every source radar frame");
assert.equal(countImages("tile.openstreetmap.org"), baseBeforeZoomBack, "zoom-back does not re-fetch the cached regional base");
assert.equal(fetches.filter(value => value === "https://api.rainviewer.com/public/weather-maps.json").length, jsonBeforeZoomBack, "zoom-back does not re-fetch the RainViewer frame list");

const retainedFrames=radar.RADAR_STATE.liteFrames.slice();
radar.RADAR_STATE.liteLastRefreshAt=0;
failNextImage=url=>String(url).includes("radar.example/latest");
assert.equal(await radar.radarRefreshLite(),false,"one missing newest-frame tile rejects a partial replacement");
assert.ok(retainedFrames.every((frame,index)=>radar.RADAR_STATE.liteFrames[index]===frame&&!frame.closed),"partial replacement keeps the prior complete view visible");
assert.match(radar.RADAR_STATE.error,/complete radar coverage/i,"partial replacement reports a retryable complete-coverage error");

slowFollowupFrames=true;
const rapidIn=radar.radarLiteToggleZoom();
const rapidOut=radar.radarLiteToggleZoom();
await Promise.all([rapidIn,rapidOut]);
assert.equal(radar.RADAR_STATE.liteZoom,6,"rapid zoom requests leave only the final regional generation active");
assert.equal(nodes.get("radarlitecanvas").hidden,false,"final rapid zoom keeps a complete visible snapshot");
assert.equal(radar.RADAR_STATE.litePlaying,false,"zoom leaves Lite animation paused");

radar.RADAR_STATE.liteLastRefreshAt=0;
slowAllTiles=true;
const closingBuild=radar.radarRefreshLite();
await waitFor(()=>radar.RADAR_STATE.liteRequests.size>0,"a cancellable Lite replacement request");
radar.closeRadar();
await closingBuild;
await delay(20);
assert.equal(nodes.get("radarlitecanvas").width, 1, "close releases the visible canvas backing store");
assert.equal(nodes.get("radarlitecanvas").hidden, true, "close hides the old snapshot");
assert.equal(radar.RADAR_STATE.liteScratch, null, "close releases the scratch canvas");
assert.equal(radar.RADAR_STATE.liteBase, null, "close releases the shared base canvas");
assert.deepEqual(Object.keys(radar.RADAR_STATE.liteBaseCache), [], "close releases the per-zoom base cache");
assert.equal(radar.RADAR_STATE.liteFrames.length, 0, "close releases all Lite frame references");
assert.ok(bitmaps.every(bitmap => bitmap.closed), "every ImageBitmap is closed across rebuilds and close");

context.CONFIG.profile = "lite";
nodes.get("radarstage").clientHeight = 720;
assert.equal(radar.radarLitePx(), 640, "Lite uses the fixed source-max canvas cap when space allows");
assert.equal(radar.radarLiteFrameLimitForZoom(7), 1, "closed radar returns to an idle one-frame count until the next source timeline loads");
assert.equal(radar.radarFrameCount("rainviewer"), 1, "closed radar resets to a one-frame idle count until the next source timeline loads");

console.log("PASS: Lite radar covers every viewport edge, rejects partial replacements, cancels stale generations, and retains the complete source timeline");
