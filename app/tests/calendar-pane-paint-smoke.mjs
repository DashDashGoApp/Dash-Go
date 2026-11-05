#!/usr/bin/env node
// The calendar pane fades its last rows instead of cutting glyphs. That fade
// must be a static overlay, not a mask-image on the scrolling container: on the
// Pi (WEBKIT_DISABLE_COMPOSITING_MODE=1) a mask on a scroller re-rasterizes the
// whole masked subtree on every repaint, which made event tags flicker and land
// a pixel off whenever the rotating message repainted.
import assert from "node:assert/strict";
import {readFileSync} from "node:fs";
import {resolve} from "node:path";

const root = resolve(process.argv[2] || ".");
const read = (rel) => readFileSync(resolve(root, rel), "utf8");
const calendar = read("ui/css/dashboard/calendar.css");
const dock = read("ui/js/lists-dock.js");

assert.doesNotMatch(calendar, /#calscroll\{[^}]*mask-image/,
  "the calendar scroller must not carry a mask-image (it re-rasterizes the whole grid on every repaint)");
assert.doesNotMatch(calendar, /#calscroll\{[^}]*-webkit-mask-image/,
  "the calendar scroller must not carry a -webkit-mask-image either");

const overlay = calendar.match(/#calwrap::after\{[^}]*\}/)?.[0] || "";
assert.ok(overlay, "the calendar pane must keep an edge fade as an overlay");
assert.match(overlay, /position:absolute/, "the edge fade must be positioned over the pane");
assert.match(overlay, /height:26px/, "the edge fade must keep its 26px band");
assert.match(overlay, /pointer-events:none/, "the edge fade must never eat touch input");
assert.match(overlay, /linear-gradient\(to bottom,rgba\(0,0,0,0\),var\(--bg\)\)/,
  "the edge fade must resolve to the pane background colour, exactly like the old mask did");

const snapback = calendar.match(/#snapback\{[^}]*\}/)?.[0] || "";
const snapZ = Number((snapback.match(/z-index:(\d+)/) || [])[1]);
const fadeZ = Number((overlay.match(/z-index:(\d+)/) || [])[1]);
assert.ok(snapZ > fadeZ, "the TODAY button must stay above the edge fade");

// Lists dock visibility is repainted on every stream frame; the shell must not
// re-write identical hidden/aria-hidden attributes (a style invalidation).
assert.match(dock, /if\(shell\.hidden!==!visible\)shell\.hidden=!visible;/,
  "Lists dock visibility must write the hidden attribute only on change");
assert.match(dock, /if\(shell\.getAttribute\("aria-hidden"\)!==aria\)shell\.setAttribute\("aria-hidden",aria\);/,
  "Lists dock visibility must write aria-hidden only on change");

console.log("PASS: calendar edge fade is a static overlay and Lists dock visibility writes only on change");
