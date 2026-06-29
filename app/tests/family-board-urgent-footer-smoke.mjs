#!/usr/bin/env node
// beta.43 Family Board contract: urgent household communication has a separate
// dashboard lane from operational health warnings, exact expiry, and no poll.
import assert from "node:assert/strict";
import {readFileSync} from "node:fs";
import {resolve} from "node:path";

const root=resolve(process.argv[2]||".");
const read=rel=>readFileSync(resolve(root,rel),"utf8");
const store=read("internal/household/family/store.go");
const mutations=read("internal/household/family/mutations.go");
const summary=read("internal/household/family/summary.go");
const core=read("ui/family-board-core.js");
const board=read("ui/family-board.js");
const footer=read("ui/js/family-board-footer.js");
const footerCss=read("ui/css/dashboard/family-board-footer.css");
const messageCss=read("ui/css/dashboard/sidebar-weather-messages.css");
const health=read("ui/js/display-health.js");

assert.match(store,/showUrgentAlertsOnDashboard/,"Family Board must store the urgent-only dashboard preference");
assert.match(store,/showPinnedOnDashboard/,"legacy pinned-footer preference needs migration compatibility");
assert.match(summary,/"displayMode"/,"server summary must select an explicit alert mode");
assert.match(summary,/"alert"/,"urgent unpinned notes must have compact alert mode");
assert.match(summary,/"message"/,"urgent pinned notes must have wrapped message mode");
assert.match(summary,/"nextUrgentExpiryAt"/,"summary must arm only the next urgent expiry");
assert.match(store,/func ExpiryAtEndOfLocalDate/,"date-choice expiry must preserve through-date local semantics");
assert.match(store,/"expiresAt"/,"durable notes must use exact expiration timestamps");
assert.match(mutations,/case "duration"/,"mutations must support duration expiry");
assert.match(mutations,/case "date"/,"mutations must support date expiry");
assert.match(mutations,/"minutes"/,"duration expiry must support minutes");
assert.match(mutations,/"hours"/,"duration expiry must support hours");
assert.match(mutations,/minute expiration must be between 1 and 1440/,"minute duration needs a bounded server rule");
assert.match(mutations,/hour expiration must be between 1 and 168/,"hour duration needs a bounded server rule");
assert.match(mutations,/case "keep"/,"edits must preserve existing exact expiry until the user changes it");
assert.match(core,/function expirationLabel/,"Family Board must render exact expiry labels");
assert.match(board,/After a duration/,"Family Board form must expose duration expiry");
assert.match(board,/Quick choices/,"Family Board form needs touch-friendly expiry presets");
assert.match(board,/Pin as the dashboard message when urgent/,"pin wording must describe the urgent-only alert contract");
assert.match(board,/Urgent Family Board alerts/,"settings UI must describe the new dashboard behavior");
assert.ok(!/showPinnedOnDashboard/.test(board),"new Family Board UI must not save the retired pinned-footer setting");
assert.match(footer,/familyBoardFooterMode/,"footer must choose none, alert, or message modes");
assert.match(footer,/mode==="alert"/,"footer needs compact urgent alert mode");
assert.match(footer,/mode==="message"/,"footer needs wrapped pinned urgent message mode");
assert.match(footer,/mark\.textContent="!"/,"compact alert must visibly be only an exclamation point");
assert.match(footer,/nextUrgentExpiryAt/,"footer timer must use urgent expiry only");
assert.ok(!footer.includes("setInterval"),"footer may not poll");
assert.match(footer,/healthVisible/,"compact geometry must account for the independent health warning lane");
assert.match(health,/familyBoardFooterReflow/,"health warning changes must reflow, not overlap, the Family Board lane");
assert.ok(!/position\s*:\s*fixed/.test(footerCss),"Family Board alert must remain in-flow");
assert.match(footerCss,/\.mode-alert/,"compact exclamation mode needs dedicated CSS");
assert.match(footerCss,/\.mode-message/,"wrapped message mode needs dedicated CSS");
assert.match(footerCss,/white-space:nowrap/,"only compact metadata may stay on one line");
assert.match(footerCss,/overflow-wrap:anywhere/,"urgent pinned text must wrap rather than ellipsis");
assert.match(footerCss,/-webkit-line-clamp:4/,"desktop urgent message is bounded but can show multiple lines");
assert.match(footerCss,/-webkit-line-clamp:3/,"compact urgent message keeps a safe multi-line bound");
assert.match(messageCss,/#stale/,"health warning remains in the dashboard message bar");
assert.match(messageCss,/margin-left:auto/,"health warning keeps its independent right-side lane");
console.log("PASS: Family Board urgent dashboard alerts use exact expiry and an in-flow lane independent from health warnings");
