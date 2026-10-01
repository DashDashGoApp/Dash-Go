import assert from "node:assert/strict";
import fs from "node:fs";

// Guards the rotating-message catalog and its provider adapters. A provider that
// is listed in a category but missing from providerLabels or the dispatcher
// fails silently at runtime as "unknown provider", and a category without a
// local pool stops rotating the moment the network is unavailable.
const read=(path)=>fs.readFileSync(path,"utf8");
const core=read("internal/messages/messages_core.go");
const providers=read("internal/messages/messages_providers.go");
const jokes=read("internal/messages/messages_providers_jokes.go");
const quotes=read("internal/messages/messages_providers_quotes.go");
const facts=read("internal/messages/messages_providers_facts.go");
const history=read("internal/messages/messages_providers_history.go");
const refresh=read("internal/messages/messages_refresh.go");

const categoryLine=/^\t\{"([a-z-]+)", "([^"]+)", "([^"]+)", \[\]string\{([^}]*)\}, "([a-z-]+)", (true|false)\},$/gm;
const categories=[...core.matchAll(categoryLine)].map((m)=>({
  id:m[1],
  label:m[2],
  providers:[...m[4].matchAll(/"([^"]+)"/g)].map((x)=>x[1]),
  local:m[5],
  nsfw:m[6]==="true",
}));
assert.ok(categories.length>=9,`message catalog shrank to ${categories.length} categories`);
const ids=new Set(categories.map((c)=>c.id));
for(const required of ["jokes","quotes","facts","history","trivia","riddles","wellbeing","family","nsfw-jokes"]){
  assert.ok(ids.has(required),`message category ${required} is missing`);
}

const labels=new Set([...core.matchAll(/"([a-z_]+)": "[^"]+"/g)].map((m)=>m[1]));
const dispatch=new Set([...providers.matchAll(/"([a-z_]+)"/g)].map((m)=>m[1]));
for(const category of categories){
  assert.ok(category.providers.length>0,`category ${category.id} has no providers`);
  assert.ok(new RegExp(`"${category.local}":\\s*\\{`).test(core),`category ${category.id} has no local fallback pool`);
  for(const provider of category.providers){
    assert.ok(labels.has(provider),`provider ${provider} has no providerLabels entry`);
    assert.ok(dispatch.has(provider),`provider ${provider} has no fetchMessageProvider dispatch branch`);
  }
  if(category.nsfw){
    assert.ok(category.providers.every((p)=>/nsfw|adult/.test(p)),`NSFW category ${category.id} must use an adult-only provider`);
  }
}

// The two providers that answered nothing during the 1.5.13 audit must not lead a
// chain, and the dead one must not return at all.
assert.doesNotMatch(core,/numbersapi_https/,"the dead Numbers API provider was restored");
assert.doesNotMatch(core,/Numbers API HTTPS/,"the dead Numbers API label was restored");
const quoteCategory=categories.find((c)=>c.id==="quotes");
assert.equal(quoteCategory.providers[0],"dummyjson_quotes","a verified live provider must lead the quotes chain");
assert.ok(quoteCategory.providers.includes("quotable"),"the dead quotable host stays as a last-resort fallback");

// Local pools must never invent content: the history pool carries prompts only,
// with no year that could be read as a fabricated anniversary.
const historyPool=core.match(/"history":\s*\{([^}]*)\}/);
assert.ok(historyPool,"the history local pool is missing");
assert.doesNotMatch(historyPool[1],/\b(1[0-9]{3}|20[0-9]{2})\b/,"the history fallback pool must not invent dated anniversaries");
const triviaPool=core.match(/"trivia":\s*\{([^}]*)\}/);
assert.ok(triviaPool,"the trivia local pool is missing");
assert.ok([...triviaPool[1].matchAll(/Answer:/g)].length>=14,"the trivia fallback pool needs a stated answer for every entry");

// Hourly history and trivia both depend on keyless public sources, and every
// adapter must be HTTPS.
assert.match(core,/"history", "This day in history"/);
assert.match(core,/"trivia", "Trivia"/);
assert.match(history,/api\.wikimedia\.org\/feed\/v1\/wikipedia\/en\/onthisday/);
assert.match(history,/opentdb\.com\/api\.php\?amount=/);
assert.match(history,/encode=url3986/,"Open Trivia DB must be requested in a decodable encoding");
assert.match(history,/opentdbSafeCategories/,"trivia must stay inside the household-safe category list");
assert.match(jokes,/"nsfw,religious,political,racist,sexist,explicit"/,"the clean joke feed must keep its adult-content blacklist");
assert.doesNotMatch(jokes,/chucknorris/,"a verified crude joke source must not return");
assert.doesNotMatch(core,/chucknorris/);
assert.match(quotes,/api\.quotable\.kurokeita\.dev/);
assert.match(quotes,/stoic-quotes\.com/);
assert.match(quotes,/thequoteshub\.com/);
assert.match(quotes,/tickerQuoteMaxRunes/,"a mixed-length quote feed must bound its line length");
assert.match(facts,/uselessfacts\.jsph\.pl/);
for(const [name,source] of [["core",core],["providers",providers],["jokes",jokes],["quotes",quotes],["facts",facts],["history",history]]){
  assert.doesNotMatch(source,/"http:\/\//,`${name} must not use a plaintext message provider URL`);
}

// One bounded deadline covers the whole refresh so a slow category chain cannot
// stretch the rotating-message section.
assert.match(refresh,/messageRefreshBudget = 25 \* time\.Second/);
assert.match(refresh,/context\.WithTimeout\(ctx, messageRefreshBudget\)/);
assert.match(refresh,/s\.fetchMessageCategory\(bounded,/);

console.log(`PASS: message source catalog (${categories.length} categories, ${labels.size} labeled providers, dispatcher complete, local pools intact)`);
