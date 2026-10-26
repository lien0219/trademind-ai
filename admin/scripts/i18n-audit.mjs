#!/usr/bin/env node
// Internationalization debt inventory. No external translation service or
// runtime network request: this scans every maintained frontend source file.
import { readdirSync, readFileSync } from 'node:fs';
import { join, relative, resolve } from 'node:path';

const root=resolve('src');
const exclude=/(__tests__|\.test\.|\.spec\.|\.d\.ts$|\.generated\.|\/i18n\/dictionaries\.ts$)/;
const chinese=/[\u3400-\u9fff]/;
const userFacing=/\b(label|title|subTitle|placeholder|message|description|text|tooltip|aria-label|okText|cancelText|name)\s*[:=]\s*['"`]|message\.(?:success|error|warning|info)\s*\(|>\s*[^<{]*[\u3400-\u9fff]/;
let files=0;
const gaps=[];
function walk(dir) {
  for(const item of readdirSync(dir,{withFileTypes:true})) {
    const path=join(dir,item.name);
    if(item.isDirectory()) {if(!exclude.test(path))walk(path);continue;}
    if(!/\.[jt]sx?$/.test(item.name)||exclude.test(path)) continue;
    files++;
    const lines=readFileSync(path,'utf8').split(/\r?\n/);
    lines.forEach((line,i)=>{
      if(chinese.test(line)&&userFacing.test(line)&&!line.trim().startsWith('//')){
        gaps.push({file:relative(root,path).replaceAll('\\','/'),line:i+1,sample:line.trim().slice(0,130)});
      }
    });
  }
}
walk(root);
console.log(JSON.stringify({sourceFiles:files,unmigratedTextSites:gaps.length,affectedFiles:new Set(gaps.map(x=>x.file)).size,samples:gaps.slice(0,35)},null,2));
if(process.argv.includes('--strict')&&gaps.length)process.exitCode=1;
