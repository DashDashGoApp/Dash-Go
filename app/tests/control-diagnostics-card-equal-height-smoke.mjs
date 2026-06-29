#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import {fileURLToPath} from "node:url";

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const read=p=>fs.readFileSync(path.join(root,p),"utf8");
const ia=read("ui/css/control/information-architecture.css");
const diagnostics=read("ui/js/control-calendars-logs.js");

function block(source,name){
  const at=source.indexOf(`function ${name}(`);assert.notEqual(at,-1,`missing ${name}`);
  const open=source.indexOf("{",at);let depth=0;
  for(let i=open;i<source.length;i++){
    if(source[i]==="{")depth++;
    else if(source[i]==="}"&&!--depth)return source.slice(at,i+1);
  }
  throw new Error(`unterminated ${name}`);
}

const desktop=ia.match(/@media\(min-width:1280px\)\{([\s\S]*?)\n\}/);
assert.ok(desktop,"missing desktop Control-pair media block");
const rules=desktop[1];
assert.match(rules,/#ctrlscreensettings \.ctrlcardrow-screen,#ctrldiag \.ctrlcardrow-doctor\{grid-template-columns:repeat\(2,minmax\(0,1fr\)\);align-items:stretch;\}/,"Diagnostics must share the desktop equal-height peer wrapper");
assert.match(rules,/#ctrlscreensettings \.ctrlcardrow-screen>\.actiongroup,#ctrldiag \.ctrlcardrow-doctor>\.actiongroup\{display:flex;flex-direction:column;min-height:0;\}/,"each Diagnostics peer panel must accept the matched desktop height");
assert.match(rules,/#ctrlscreensettings \.ctrlcardrow-screen>\.actiongroup>\.actiongroup-grid,#ctrldiag \.ctrlcardrow-doctor>\.actiongroup>\.actiongroup-grid\{flex:1 1 auto;min-height:0;\}/,"each Diagnostics action grid must fill the matched peer panel");
assert.ok(!/#ctrldiag \.ctrlcardrow-doctor\{[^}]*align-items:start;/.test(rules),"Diagnostics must not retain a start-aligned unequal-height override");

const diag=block(diagnostics,"renderCtrlDiagnostics");
assert.match(diag,/const actionRow=el\("div","ctrlcardrow ctrlcardrow-doctor"\)/,"Diagnostics must retain its dedicated pair wrapper");
assert.ok(diag.includes("actionRow.append(repair.group,support.group)"),"Inspect & repair and Diagnostics bundle must remain the two wrapper peers");
assert.match(diag,/doctor-actiongroup-repair[\s\S]*doctor-actiongroup-support/,"the existing 3-up repair and 2-up support group roles must remain intact");
console.log("PASS: beta.29 keeps desktop Diagnostics peer panels and their action grids at one matched height without changing the narrow one-column flow");
