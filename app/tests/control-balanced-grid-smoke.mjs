#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=rel=>fs.readFileSync(path.join(root,rel),"utf8");
const ui=read("ui/js/control-ui.js");
const actions=read("ui/js/control-system-actions.js");
const visuals=read("ui/js/control-visual-style.js");
const themes=read("ui/js/control-theme.js");
const layout=read("ui/css/control/layout.css");
const themeCss=read("ui/css/dashboard/control-theme-actions-lite.css");

assert.match(ui,/function ctrlApplyBalancedGridCount\(grid\)/,"Control needs an explicit opt-in grid count helper");
assert.match(ui,/ctrl-balanced-grid","ctrl-grid-count-"/,"count helper must mark a semantic balanced grid class");
assert.match(actions,/ctrlApplyBalancedGridCount\(common\.grid\);/,"Quick actions must opt into intentional count geometry");
assert.match(actions,/ctrlApplyBalancedGridCount\(danger\.grid\);/,"Power actions must opt into intentional count geometry");
assert.match(visuals,/ctrlApplyBalancedGridCount\(icons\.grid\);/,"Weather SVG choices must opt into intentional count geometry");
assert.match(visuals,/ctrlApplyBalancedGridCount\(decor\.grid\);/,"Seasonal decor choices must opt into intentional count geometry");
assert.match(layout,/#ctrlactions\.actiongrid\{display:block;min-height:0;/,"Quick actions must not retain a flex growth lane");
assert.match(layout,/ctrl-grid-count-5\{grid-template-columns:repeat\(6,minmax\(0,1fr\)\);\}/,"five Control choices need a six-track centered canvas");
assert.match(layout,/ctrl-grid-count-5>:nth-child\(4\)\{grid-column:2 \/ span 2;\}/,"five Control choices need the fourth item centered under a 3-item row");
assert.match(layout,/ctrl-grid-count-5>:nth-child\(5\)\{grid-column:4 \/ span 2;\}/,"five Control choices need the fifth item centered under a 3-item row");
assert.match(layout,/ctrl-grid-count-6\{grid-template-columns:repeat\(3,minmax\(0,1fr\)\);\}/,"six Control choices must form 3 + 3 instead of 5 + 1");
assert.match(layout,/@media\(max-width:760px\)[\s\S]*ctrl-balanced-grid:not\(\.ctrl-grid-count-1\)\{grid-template-columns:repeat\(2,minmax\(0,1fr\)\);\}/,"compact Control must retain two touch-safe columns at the shared small-screen tier");
assert.match(themes,/cards\.dataset\.themeCount=String\(names\.length\);/,"theme groups must expose their rendered count");
assert.match(themes,/theme-count-"\+names\.length/,"five and six preview groups must use count-aware classes");
assert.match(themeCss,/themegroupcards\.theme-count-5\{grid-template-columns:repeat\(6,minmax\(0,1fr\)\);\}/,"five theme previews need centered 3 + 2 geometry");
assert.match(themeCss,/themegroupcards\.theme-count-6\{grid-template-columns:repeat\(3,minmax\(0,1fr\)\);\}/,"six theme previews need 3 + 3 geometry");
console.log("PASS: count-aware Control and theme grids avoid stranded fifth/sixth choices and Quick actions remains intrinsic.");
