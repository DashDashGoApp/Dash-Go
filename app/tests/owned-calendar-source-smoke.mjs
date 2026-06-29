import assert from "node:assert/strict";
import {readFileSync} from "node:fs";
import {resolve} from "node:path";

const root=resolve(process.argv[2]||".");
const read=rel=>readFileSync(resolve(root,rel),"utf8");
const sources=read("internal/calendar/events/sources.go");
const owned=read("internal/calendar/manifest.go");
const manifest=owned;
const cache=read("internal/calendar/events/types.go");
const browserCache=read("ui/js/event-cache.js");

assert.match(owned,/calendars\/chore-wheel\.ics[\s\S]*?Name: "Chores", Color: "#7fc4c4", Owner: "chore-wheel"/,"Chore Wheel must have one canonical Chores feed identity and owner");
assert.match(owned,/calendars\/maintenance\.ics[\s\S]*?Name: "Maintenance", Color: "#d9c074", Owner: "maintenance"/,"Maintenance must have one canonical feed identity and owner");
assert.match(owned,/calendars\/routines\.ics[\s\S]*?Name: "Routines", Color: "#a999d4", Owner: "routines"/,"Routines must have one canonical feed identity and owner");
assert.match(manifest,/if owned, ok := OwnedSource\(url\); ok \{[\s\S]*?display, color, tag, owner = owned\.Name, owned\.Color, owned\.Tag, owned\.Owner/,"manifest must use canonical app-owned calendar metadata");
assert.match(sources,/seen, out := map\[string\]bool\{\}, \[\]CalendarSource\{\}/,"configured sources must deduplicate by canonical identity");
assert.match(sources,/seen\[key\] = true[\s\S]*?if !calendarEntryEnabled\(m\)/,"an explicit disabled source must block app feed reinjection");
assert.doesNotMatch(sources,/chore-wheel\.ics/,"Chore Wheel feed must not be appended as a duplicate runtime source");
assert.doesNotMatch(sources,/maintenance\.ics/,"Maintenance feed must not be appended as a duplicate runtime source");
assert.match(cache,/const CacheVersion = 5/,"calendar source repair requires a new server cache version");
assert.match(browserCache,/cache\.version!==5/,"browser must reject pre-repair duplicate event caches");
console.log("owned calendar source smoke: canonical app feeds, explicit ownership, visibility, de-duplication, and cache invalidation hold");
