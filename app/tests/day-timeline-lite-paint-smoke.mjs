#!/usr/bin/env node
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import {fileURLToPath} from "node:url";
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const source=fs.readFileSync(path.join(root,"ui/js/event-popup.js"),"utf8");
const values={"--bg":"#0a0a0d","--panel":"rgba(255,255,255,0.025)","--accent":"#7fd6a8"};
const rootNode={getAttribute(){return "night";}};
const context=vm.createContext({
  console,Math,String,Number,Date,Map,
  document:{documentElement:rootNode},
  getComputedStyle(){return {getPropertyValue:key=>values[key]||""};},
  dtLiteDayPopupProfile:()=>true
});
vm.runInContext(source+"\nglobalThis.__paint={dtParseColor,dtPanelBaseRGB,dtOpaque,dtBeginDayCardPaintContext,dtApplyCardColor};",context,{filename:"day-event-source.js"});
const {dtParseColor,dtPanelBaseRGB,dtOpaque,dtBeginDayCardPaintContext,dtApplyCardColor}=context.__paint;
assert.deepEqual([...dtParseColor("#abc").rgb],[170,187,204],"short hex parser preserves event colors");
assert.deepEqual([...dtPanelBaseRGB({getPropertyValue:key=>values[key]||""})],[16,16,19],"translucent panel resolves once over opaque dashboard background");
assert.equal(dtOpaque([255,0,0],[16,16,19],0.17),"rgb(57,13,16)","event color blend is emitted as an opaque RGB fill");
const model={};dtBeginDayCardPaintContext(model);
assert.equal(model.dtCardPaint.lite,true,"Lite popup records one reusable paint context");
const style={values:{},setProperty(k,v){this.values[k]=v;}};
const card={style};dtApplyCardColor(card,"#ff0000","timeline",model);
assert.match(card.style.backgroundColor,/^rgb\(/,"Lite timeline card background must be opaque");
assert.match(card.style.borderColor,/^rgb\(/,"Lite timeline card border must be opaque");
assert.equal(card.style.borderLeftColor,"#ff0000","accent identity stays on the left border");
console.log("PASS: Lite timeline card colors are precomputed opaque blends over the popup panel");
