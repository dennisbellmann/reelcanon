import { spawn, spawnSync } from "node:child_process";
import { mkdirSync, openSync } from "node:fs";
import { randomBytes } from "node:crypto";
import path from "node:path";
import { fileURLToPath } from "node:url";
const root=fileURLToPath(new URL("../",import.meta.url));
const database=process.env.TEST_DATABASE_URL;
if (!database || !new URL(database).pathname.endsWith("_test")) throw new Error("Set TEST_DATABASE_URL to a PostgreSQL database whose name ends in _test.");
const schema=`e2e_${Date.now()}`;
const testURL=new URL(database);testURL.searchParams.set("search_path",schema);
const env={...process.env,DATABASE_URL:testURL.href,API_DATABASE_URL:testURL.href,ADMIN_DATABASE_URL:testURL.href,API_ADDR:"127.0.0.1:8090",ADMIN_ADDR:"127.0.0.1:8091",ADMIN_USER:"e2e-admin",ADMIN_PASSWORD:randomBytes(24).toString("hex"),TMDB_API_KEY:"",TMDB_READ_ACCESS_TOKEN:"",FRONTEND_DIST:path.join(root,"frontend/dist"),E2E_BASE_URL:"http://127.0.0.1:8090",E2E_ADMIN_URL:"http://127.0.0.1:8091"};
function run(command,args,options={}){const result=spawnSync(command,args,{cwd:root,env,stdio:"inherit",...options});if(result.error)throw result.error;if(result.status!==0)throw new Error(`${command} exited ${result.status}`);}
run("node",["scripts/build-backend.mjs"]);
run(process.execPath,["frontend/node_modules/typescript/bin/tsc","-b","frontend"]);
run(process.execPath,[path.join(root,"frontend/node_modules/vite/bin/vite.js"),"build"],{cwd:path.join(root,"frontend")});
const extension=process.platform==="win32"?".exe":"";
const binary=name=>path.join(root,"backend/bin",name+extension);
const seed=args=>run(binary("test-seed"),args,{cwd:path.join(root,"backend")});
mkdirSync(path.join(root,".local"),{recursive:true});
const children=[];
let status=1;
try{
 seed(["-schema",schema]);
 for(const name of ["api","admin"]){const log=openSync(path.join(root,".local",`e2e-${name}.log`),"w");children.push(spawn(binary(name),[],{cwd:path.join(root,"backend"),env,stdio:["ignore",log,log],windowsHide:true}));}
 let ready=false;
 for(let attempt=0;attempt<60;attempt++){try{const response=await fetch(`${env.E2E_BASE_URL}/api/v1/healthz`);if(response.ok){ready=true;break;}}catch{}await new Promise(resolve=>setTimeout(resolve,250));}
 if(!ready)throw new Error("E2E API did not become ready; inspect .local/e2e-api.log");
 const playwright=path.join(root,"frontend/node_modules/@playwright/test/cli.js");
 const child=spawn(process.execPath,[playwright,"test",...process.argv.slice(2)],{cwd:path.join(root,"frontend"),env,stdio:"inherit",windowsHide:true});
 status=await new Promise((resolve,reject)=>{child.once("error",reject);child.once("exit",code=>resolve(code??1));});
}finally{
 await Promise.all(children.map(child=>new Promise(resolve=>{if(child.exitCode!==null){resolve();return;}child.once("exit",resolve);child.kill();})));
 seed(["-schema",schema,"-cleanup"]);
}
process.exitCode=status;
