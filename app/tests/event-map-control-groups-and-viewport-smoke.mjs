#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=p=>fs.readFileSync(path.join(root,p),"utf8");
const js=read("ui/js/event-maps.js");
const css=read("ui/css/dashboard/event-popup-compact.css");
const sharedCss=read("ui/css/dashboard/popups-alerts-maps.css");
const maps=read("internal/maps/maps_render.go");

assert.equal((js.match(/\{key:"(?:area|street|close)", label:/g)||[]).length,3,"event maps must retain exactly three released detail choices");
assert.equal((js.match(/\{key:"(?:standard|hybrid)", label:/g)||[]).length,2,"event maps must retain exactly two released style choices");
for(const token of [
  'addControlGroup("mapstylegroup","Map style","mapcontrolrow--style",EVENT_MAP_STYLES,"mapstyle")',
  'addControlGroup("mapzoomgroup","Map detail","mapcontrolrow--zoom",EVENT_MAP_ZOOMS,"mapzoom")',
  'row.setAttribute("role","radiogroup")',
  'row.setAttribute("aria-label",heading)',
  'button.setAttribute("role","radio")',
  'button.setAttribute("aria-checked","false")',
  'function syncMapControlState()',
  'button.classList.toggle("on",selected)',
  'button.setAttribute("aria-checked",String(selected))',
  'bindEventMapRadioKeys(row,buttons)',
  'buttons[next].click()',
]) assert.ok(js.includes(token),`missing grouped event-map control contract: ${token}`);
assert.ok(!/for\(const sdef of EVENT_MAP_STYLES\)\{[^}]*controls\.appendChild/.test(js),"styles may not append directly into one flat toolbar");
assert.ok(!/for\(const zdef of EVENT_MAP_ZOOMS\)\{[^}]*controls\.appendChild/.test(js),"details may not append directly into one flat toolbar");

for(const token of [
  '.mapcontrolrow--style{grid-template-columns:repeat(2,minmax(0,1fr));}',
  '.mapcontrolrow--zoom{grid-template-columns:repeat(3,minmax(0,1fr));}',
  '.mapcontrolgroup + .mapcontrolgroup',
  'min-height:44px',
  '@media (min-width:641px) and (min-height:701px)',
  'height:auto;min-height:0;max-height:none;aspect-ratio:26/11;',
  '#pop.eventpop .eventmap img{object-fit:contain;}',
  '@media (max-height:700px), (max-width:640px)',
  'height:clamp(154px,28vh,208px);min-height:0;max-height:none;aspect-ratio:auto;',
]) assert.ok(css.includes(token),`missing grouped viewport CSS contract: ${token}`);
assert.ok(!css.includes('repeat(5,minmax(0,1fr))'),"event popup may not return to a combined five-button toolbar");
assert.ok(!css.includes('object-fit:cover'),"event popup override may not reintroduce map crop behavior");
assert.ok(sharedCss.includes('.eventmap .mapstage{position:relative;width:100%;aspect-ratio:26/11;'),"shared static-map stage must retain the native backend ratio");
for(const token of [
  'renderTileSVG(p, lat, lon, zoom, 520, 220)',
  'renderLayeredTileSVG(p, lat, lon, zoom, 520, 220)',
  'renderArcGISExportSVG(lat, lon, zoom, 520, 220)',
  'size=520x220',
]) assert.ok(maps.includes(token),`backend static-map dimensions/cache contract changed unexpectedly: ${token}`);
console.log("PASS: event popup keeps independent 2-up style and 3-up detail controls with a full-image 26:11 map viewport and unchanged 520x220 backend payload.");
