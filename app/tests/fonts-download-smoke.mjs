#!/usr/bin/env node
import fs from "node:fs"; import path from "node:path"; import {fileURLToPath} from "node:url";
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),".."); const read=p=>fs.readFileSync(path.join(root,p),"utf8"); const must=(v,m)=>{if(!v)throw new Error(m)};
const index=read("index.html"), runtime=read("ui/js/settings-runtime.js"), editor=read("ui/js/control-dashboard-typography.js"), css=read("ui/css/dashboard/base.css"), server=[read("cmd/dashboard-control-server/fonts_http.go"),read("internal/settings/fonts.go")].join("\n"), installer=read("../installer/install.sh"), doctor=read("bin/doctor.sh");
must(index.includes('id="dynfonts"'),"dynamic font stylesheet link missing");
for(const token of ["Nunito","Atkinson Hyperlegible","refreshDashboardFontStatus","dashboardFontAvailable","DASHBOARD_FONT_DOWNLOADS","downloadDashboardFont"]) must(runtime.includes(token),"runtime font contract missing "+token);
must(editor.includes('missing-font')&&editor.includes(' ↓'),"editor must expose a missing-font recovery affordance");
must(/dashboardFontInfo\(next\)\.state==="missing"[\s\S]*?await downloadDashboardFont\(next\)/.test(editor),"selecting a missing font must begin the guarded download in the same action");
must(!runtime.includes("armDashboardFontDownload"),"stale second-tap download gate must not return");
must(css.includes("'Libre Franklin Fallback'"),"base fallback must remain explicit");
for(const token of ["/api/fonts/status","/api/fonts/face.css","/api/fonts/download","runtimeFontSpecs","fontLooksValid","runtimeFontAssetValid","RuntimeFontSHA256","LimitReader","font integrity check failed"]) must(server.includes(token),"server font route missing "+token);
must(!server.includes("raw.githubusercontent.com/google/fonts/main/"),"runtime fonts must not use Google Fonts main branch URLs");
must(/SHA256:\s*"[a-f0-9]{64}"/.test(server),"runtime font assets require baked-in SHA-256 values");
must(server.includes("downloadRuntimeFontWithClient"),"runtime font download must stage before replacing live files");
for(const token of ["ensure_dashboard_fonts(){","font_file_valid(){","font_download(){",".font-stage.","RUNTIME_FONT_DIR","LibreFranklin-400.ttf","DMMono-500.ttf"]) must(installer.includes(token),"installer font recovery missing "+token);
must(!installer.includes('ensure_dashboard_fonts(){\n  return 0'),"font recovery must not be a no-op");
must(doctor.includes('DASHBOARD_FONTS_MISSING')&&doctor.includes('dashboard display fonts are missing'),"Doctor must surface missing dashboard fonts");
console.log("PASS: fonts are recoverable by the installer, visible to Doctor, and missing optional typography selections begin one guarded immediate download.");
