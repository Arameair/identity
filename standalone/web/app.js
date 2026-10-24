"use strict";
(()=>{
const $=id=>document.getElementById(id);let csrf="";
async function call(path,body){
  let res=await fetch(path,{method:"POST",credentials:"same-origin",headers:{"Content-Type":"application/json","X-Identity-CSRF":csrf},body:JSON.stringify(body||{})});
  return {ok:res.ok,data:await res.json().catch(()=>({}))};
}
const hidden=(id,value)=>$(id).hidden=value;
const clear=node=>node.replaceChildren();
function item(tag,content,classname){
  const x=document.createElement(tag);x.textContent=content;if(classname)x.className=classname;return x;
}
async function render(){
  let response=await fetch("/api/state",{credentials:"same-origin"});
  if(!response.ok){hidden("signedout",false);hidden("app",true);return}
  let state=await response.json(),g=state.github;
  hidden("configuration",g.status!=="not_configured");hidden("signin",g.status==="waiting"||g.status==="connected");
  hidden("waiting",g.status!=="waiting");hidden("connected",g.status!=="connected");
  hidden("features",g.status!=="connected");hidden("requests",g.status!=="connected");
  hidden("fixture",!g.test_mode);
  if(g.status==="waiting"){
    $("device-code").textContent=g.user_code||"";
    const link=g.verify_url||"";
    if(/^https:\/\/github\.com\//.test(link)||g.test_mode && /^http:\/\/127\.0\.0\.1:\d+\//.test(link)){$("github-link").href=link}
  }
  if(g.status==="connected")$("username").textContent=g.login;
  $("connect").disabled=g.status==="not_configured";
  if(g.message)$("message").textContent=g.message;

  const featureList=$("feature-list");clear(featureList);
  if(g.status==="connected")for(const f of state.catalog){
    const box=item("div","","feature"),description=document.createElement("div");
    description.append(item("h3",f.name),item("p",f.description));
    let existing=state.requests.find(r=>r.FeatureID===f.id),btn=item("button",existing?"Saved locally":"Save access request");
    btn.disabled=!!existing;
    btn.onclick=async()=>{const r=await call("/api/request",{feature:f.id});$("message").textContent=r.ok?"Saved locally — not sent to Arameair":r.data.error||"Request failed";render()};
    box.append(description,btn);featureList.append(box);
  }
  const reqList=$("request-list");clear(reqList);
  if(g.status==="connected"&&!state.requests.length)reqList.append(item("p","No requests saved on this computer."));
  for(const r of state.requests||[]){
    const box=item("div","","draft");box.append(item("strong",r.FeatureID+" — saved locally"));
    box.append(item("p",r.ID+" · Not submitted to Arameair"));
    const a=item("a","Export draft JSON");a.href="/api/export-request?feature="+encodeURIComponent(r.FeatureID);
    box.append(a);reqList.append(box);
  }
}
$("save-client").onclick=async()=>{const r=await call("/api/config/client-id",{client_id:$("client-id").value.trim()});
 $("message").textContent=r.ok?"Client ID saved. Connect GitHub next.":r.data.error||"Could not save Client ID";render()};
$("connect").onclick=async()=>{const r=await call("/api/github/start");
 if(!r.ok)$("message").textContent=r.data.error||"Could not connect to GitHub";render()};
$("cancel").onclick=async()=>{await call("/api/github/cancel");render()};
$("disconnect").onclick=async()=>{await call("/api/github/disconnect");render()};
async function boot(){
 const match=location.hash.match(/key=([0-9a-f]{48})/);
 if(match){
   history.replaceState(null,"","/");
   const response=await call("/api/session",{key:match[1]});if(response.ok)csrf=response.data.csrf;
 }
 if(!csrf){let response=await fetch("/api/session",{credentials:"same-origin"});
   if(response.ok)csrf=(await response.json()).csrf}
 if(!csrf)return;
 hidden("signedout",true);hidden("app",false);
 await render();setInterval(render,1500);
}
window.addEventListener("hashchange",()=>{if(location.hash.includes("key="))location.reload()});
boot();
})();