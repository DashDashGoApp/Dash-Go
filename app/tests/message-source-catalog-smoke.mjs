import assert from "node:assert/strict";
import fs from "node:fs";

// Guards the rotating-message catalog and its provider adapters. A provider that
// is listed in a category but missing from providerLabels or the dispatcher
// fails silently at runtime as "unknown provider", and a category without a
// local pool stops rotating the moment the network is unavailable. Since
// 1.5.21 the local pools live in the reviewable embedded local_messages.json
// dictionary (one entry per line, each with a src verification URL for the
// new facts/trivia/quotes) instead of Go map literals.
const read=(path)=>fs.readFileSync(path,"utf8");
const core=read("internal/messages/messages_core.go");
const providers=read("internal/messages/messages_providers.go");
const jokes=read("internal/messages/messages_providers_jokes.go");
const quotes=read("internal/messages/messages_providers_quotes.go");
const facts=read("internal/messages/messages_providers_facts.go");
const history=read("internal/messages/messages_providers_history.go");
const refresh=read("internal/messages/messages_refresh.go");
const localPools=JSON.parse(read("internal/messages/local_messages.json"));

const categoryLine=/^\t\{"([a-z-]+)", "([^"]+)", "([^"]+)", \[\]string\{([^}]*)\}, "([a-z-]+)", (true|false)\},$/gm;
const categories=[...core.matchAll(categoryLine)].map((m)=>({
  id:m[1],
  label:m[2],
  providers:[...m[4].matchAll(/"([^\"]+)"/g)].map((x)=>x[1]),
  local:m[5],
  nsfw:m[6]==="true",
}));
assert.ok(categories.length>=9,`message catalog shrank to ${categories.length} categories`);
const ids=new Set(categories.map((c)=>c.id));
for(const required of ["jokes","quotes","facts","history","trivia","riddles","wellbeing","family","nsfw-jokes"]){
  assert.ok(ids.has(required),`message category ${required} is missing`);
}

// The 1.5.21 additions: each new keyless source leads or backs the chain it
// joins, and a verified live provider must still lead the established chains.
const byId=Object.fromEntries(categories.map((c)=>[c.id,c]));
assert.equal(byId.trivia.providers[0],"the_trivia_api","the verified keyless trivia source must lead the trivia chain");
assert.ok(byId.trivia.providers.includes("opentdb"),"Open Trivia DB stays as the trivia fallback");
assert.equal(byId.quotes.providers[0],"dummyjson_quotes","a verified live provider must lead the quotes chain");
assert.ok(byId.quotes.providers.includes("stoic_quotes"),"the www-corrected stoic source must stay in the quotes chain");
assert.ok(byId.quotes.providers.includes("zenquotes"),"the batch quote feed must stay in the quotes chain");
assert.ok(byId.quotes.providers.includes("quotable"),"the dead quotable host stays as a last-resort fallback");
assert.ok(byId.facts.providers.includes("wikimedia_featured"),"the featured article feed must feed the facts chain");
assert.ok(byId.wellbeing.providers.includes("positive_api"),"the keyless phrase source must back the wellbeing chain");
assert.ok(byId.family.providers.includes("bored_api"),"the kid-friendly activity source must back the family chain");

const labels=new Map([...core.matchAll(/"([a-z_]+)": "([^"]+)"/g)].map((m)=>[m[1],m[2]]));
const dispatch=new Set([...providers.matchAll(/"([a-z_]+)"/g)].map((m)=>m[1]));
for(const category of categories){
  assert.ok(category.providers.length>0,`category ${category.id} has no providers`);
  assert.ok(Array.isArray(localPools[category.local]),`category ${category.id} has no local pool key in local_messages.json`);
  if(!category.nsfw){
    assert.ok(localPools[category.local].length>0,`category ${category.id} has an empty local fallback pool`);
  }
  for(const provider of category.providers){
    assert.ok(labels.has(provider),`provider ${provider} has no providerLabels entry`);
    assert.ok(dispatch.has(provider),`provider ${provider} has no fetchMessageProvider dispatch branch`);
  }
  if(category.nsfw){
    assert.ok(category.providers.every((p)=>/nsfw|adult/.test(p)),`NSFW category ${category.id} must use an adult-only provider`);
  }
}
// The four provider labels added with the 1.5.21 content.
assert.equal(labels.get("the_trivia_api"),"The Trivia API","the new trivia provider label changed");
assert.equal(labels.get("wikimedia_featured"),"Wikipedia: featured article","the new facts provider label changed");
assert.equal(labels.get("positive_api"),"Positive API","the new wellbeing provider label changed");
assert.equal(labels.get("bored_api"),"Bored API (kid-friendly)","the new family provider label changed");

// The two providers that answered nothing during the 1.5.13 audit must not lead a
// chain, and the dead one must not return at all.
assert.doesNotMatch(core,/numbersapi_https/,"the dead Numbers API provider was restored");
assert.doesNotMatch(core,/Numbers API HTTPS/,"the dead Numbers API label was restored");
assert.match(core,/"history", "trivia"/,"the shipped default must enable the two household-safe knowledge categories");

// Local pools must never invent content: the history pool carries prompts only,
// with no year that could be read as a fabricated anniversary, and the trivia
// pool states an answer for every entry.
assert.ok(localPools.history.every((e)=>!/\b(1[0-9]{3}|20[0-9]{2})\b/.test(e.text)),"the history fallback pool must not invent dated anniversaries");
assert.ok(localPools.trivia.filter((e)=>e.text.includes("Answer:")).length>=14,"the trivia fallback pool needs a stated answer for every entry");

// Hourly history and trivia both depend on keyless public sources, and every
// adapter must be HTTPS.
assert.match(core,/"history", "This day in history"/);
assert.match(core,/"trivia", "Trivia"/);
assert.match(history,/api\.wikimedia\.org\/feed\/v1\/wikipedia\/en\/onthisday/);
assert.match(history,/the-trivia-api\.com\/v2\/questions/,"the keyless trivia source must stay on its v2 questions endpoint");
assert.match(history,/opentdb\.com\/api\.php\?amount=/);
assert.match(history,/encode=url3986/,"Open Trivia DB must be requested in a decodable encoding");
assert.match(history,/opentdbSafeCategories/,"trivia must stay inside the household-safe category list");
assert.match(jokes,/"nsfw,religious,political,racist,sexist,explicit"/,"the clean joke feed must keep its adult-content blacklist");
assert.doesNotMatch(jokes,/chucknorris/,"a verified crude joke source must not return");
assert.doesNotMatch(core,/chucknorris/);
assert.match(quotes,/api\.quotable\.kurokeita\.dev/);
assert.match(quotes,/stoic-quotes\.com/);
assert.match(quotes,/thequoteshub\.com/);
assert.match(quotes,/zenquotes\.io\/api\/quotes/,"the batch quote feed must keep its attributed /api/quotes endpoint");
assert.match(quotes,/positive-api\.online\/phrase/);
assert.match(quotes,/bored-api\.appbrewery\.com\/random/);
assert.match(quotes,/"kidFriendly"/,"kid-unfriendly activity ideas must not reach the family lane");
assert.match(quotes,/tickerQuoteMaxRunes/,"a mixed-length quote feed must bound its line length");
assert.match(facts,/uselessfacts\.jsph\.pl/);
assert.match(facts,/api\.wikimedia\.org\/feed\/v1\/wikipedia\/en\/featured\//,"the featured article feed must stay on the curated daily line");
for(const [name,source] of [["core",core],["providers",providers],["jokes",jokes],["quotes",quotes],["facts",facts],["history",history]]){
  assert.doesNotMatch(source,/"http:\/\/\//,`${name} must not use a plaintext message provider URL`);
}
assert.doesNotMatch(read("internal/messages/local_messages.json"),/http:\/\//,"the local pool dictionary must not carry a plaintext verification URL");

// One bounded deadline covers the whole refresh so a slow category chain cannot
// stretch the rotating-message section.
assert.match(refresh,/messageRefreshBudget = 25 \* time\.Second/);
assert.match(refresh,/context\.WithTimeout\(ctx, messageRefreshBudget\)/);
assert.match(refresh,/s\.fetchMessageCategory\(bounded,/);

console.log(`PASS: message source catalog (${categories.length} categories, ${labels.size} labeled providers, dispatcher complete, local pools in the reviewable dictionary)`);
