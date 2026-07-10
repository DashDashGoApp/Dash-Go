#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const appRoot = path.resolve(here, "..");
const repoRoot = path.resolve(appRoot, "..");
const version = fs.readFileSync(path.join(appRoot, "VERSION"), "utf8").trim();
const changelog = fs.readFileSync(path.join(repoRoot, "CHANGELOG.md"), "utf8").replace(/\r\n?/g, "\n");
const lines = changelog.split("\n");

const canonicalCategories = new Set([
  "Features",
  "Improvements",
  "Bug fixes",
  "Performance and reliability",
  "Security",
  "Build and release integrity",
  "Removals",
  "Upgrade notes",
  "Known issues",
]);

const versionHeadingPattern = /^## \[([^\]]+)\](?:\s+[—-]\s+.+)?\s*$/;
const versionHeadings = lines
  .map((line, index) => ({ line, index, match: line.match(versionHeadingPattern) }))
  .filter((entry) => entry.match);

assert.ok(versionHeadings.length > 0, "CHANGELOG.md must contain a version heading");
assert.equal(
  versionHeadings[0].match[1],
  version,
  "the first CHANGELOG.md version section must match app/VERSION",
);
assert.equal(
  versionHeadings.filter((entry) => entry.match[1] === version).length,
  1,
  "the current version must appear exactly once as a changelog section",
);

const start = versionHeadings[0].index + 1;
const end = versionHeadings.length > 1 ? versionHeadings[1].index : lines.length;
const section = lines.slice(start, end);
assert.ok(section.some((line) => line.trim().length > 0), "the current release-note section must not be empty");

const categoryIndexes = [];
const seen = new Set();
for (let i = 0; i < section.length; i += 1) {
  const match = section[i].match(/^###\s+(.+?)\s*$/);
  if (!match) continue;
  const name = match[1];
  assert.ok(canonicalCategories.has(name), `unsupported release-note category: ${name}`);
  assert.ok(!seen.has(name), `duplicate release-note category: ${name}`);
  seen.add(name);
  categoryIndexes.push({ name, index: i });
}

assert.ok(categoryIndexes.length > 0, "the current release-note section must contain at least one canonical category");
for (let i = 0; i < categoryIndexes.length; i += 1) {
  const current = categoryIndexes[i];
  const next = i + 1 < categoryIndexes.length ? categoryIndexes[i + 1].index : section.length;
  const body = section.slice(current.index + 1, next);
  assert.ok(
    body.some((line) => /^-\s+\S/.test(line)),
    `release-note category must contain at least one bullet: ${current.name}`,
  );
}

console.log(`PASS: ${version} has a canonical current-version GitHub release-note section`);
