// Standalone Identity first-use prototype. No Project Controller dependency.
package main

import (
 "context"
 "crypto/rand"
 "crypto/subtle"
 "embed"
 "encoding/hex"
 "encoding/json"
 "errors"
 "flag"
 "fmt"
 "io"
 "net"
 "net/http"
 "os"
 "os/exec"
 "os/signal"
 "path/filepath"
 "runtime"
 "strconv"
 "strings"
 "sync"
 "syscall"
 "time"
 "runtime/debug"
)

//go:embed web/*
var assets embed.FS

const publisher="Arameair/identity"
const appVersion="0.1.0-home-gpt6-a"

var catalog=[]map[string]string{
 {"id":"thought-processor","name":"Thought Processor","description":"Control and diagnose optional thinking features","repository":"Arameair/Thought-Processor"},
 {"id":"thought-engine","name":"Thought Engine","description":"Process authorized observations","repository":"Arameair/Thought-engine"},
}

type Request struct {
 ID string
 FeatureID string
 GithubID int64
 GithubLogin string
 Status string
 CreatedAt time.Time
}
type App struct {
 mu sync.Mutex
 port int
 directory string
 key string
 cookie string
 csrf string
 gh *GitHub
 requests []Request
 ops *Operations
 started time.Time
 runID string
 revision string
 shutdown chan struct{}
 shutdownOnce sync.Once
}
func hexID(n int)string{v:=make([]byte,n);if _,e:=rand.Read(v);e!=nil{panic(e)};return hex.EncodeToString(v)}
func newApp(port int,dir string,ghcfg GitHubConfig)(*App,error){
 if err:=os.MkdirAll(dir,0700);err!=nil{return nil,err}
 a:=&App{port:port,directory:dir,key:hexID(24),cookie:hexID(32),csrf:hexID(32),gh:NewGitHub(ghcfg)}
 b,e:=os.ReadFile(filepath.Join(dir,"requests.json"));if e==nil{
  if e:=json.Unmarshal(b,&a.requests);e!=nil{return nil,fmt.Errorf("saved requests could not be read; preserved unchanged: %w",e)}
 }else if !errors.Is(e,os.ErrNotExist){return nil,e}
 ops, err := OpenOperations(dir)
 if err != nil { return nil, err }
 a.ops=ops; a.started=time.Now().UTC(); a.runID="run_"+hexID(8); a.revision="UNKNOWN"
 if b,ok:=debug.ReadBuildInfo();ok {for _,s:=range b.Settings {if s.Key=="vcs.revision" && len(s.Value)==40 {a.revision=s.Value}}}
 a.shutdown=make(chan struct{})
 if err:=os.MkdirAll(filepath.Join(dir,"runtime"),0700);err!=nil{return nil,err}
 if err:=os.WriteFile(filepath.Join(dir,"runtime","operator-code"),[]byte(a.key),0600);err!=nil{return nil,err}
 a.ops.Event("identity","started",a.runID,"RUNNING")
 return a,nil
}
func (a *App) saveRequests()error{
 b,e:=json.MarshalIndent(a.requests,"","  ");if e!=nil{return e}
 p:=filepath.Join(a.directory,"requests.json")
 if e=os.WriteFile(p+".tmp",b,0600);e!=nil{return e}
 return os.Rename(p+".tmp",p)
}
func (a *App) request(feature string)(Request,error){
 _,login:=a.gh.Token()
 a.gh.mu.Lock();id:=a.gh.userID;status:=a.gh.status;a.gh.mu.Unlock()
 if status!="connected"||id==0||login==""{return Request{},errors.New("connect GitHub first")}
 ok:=false;for _,f:=range catalog{if f["id"]==feature{ok=true;break}}
 if !ok{return Request{},errors.New("unknown feature")}
 a.mu.Lock();defer a.mu.Unlock()
 for _,r:=range a.requests{if r.FeatureID==feature&&r.GithubID==id{return r,nil}}
 r:=Request{ID:"req-"+hexID(12),FeatureID:feature,GithubID:id,GithubLogin:login,Status:"SAVED LOCALLY — NOT SUBMITTED",CreatedAt:time.Now().UTC()}
 a.requests=append(a.requests,r)
 if e:=a.saveRequests();e!=nil{a.requests=a.requests[:len(a.requests)-1];return Request{},e}
 return r,nil
}
func (a *App) currentRequests()[]Request{
 a.gh.mu.Lock();id:=a.gh.userID;status:=a.gh.status;a.gh.mu.Unlock()
 list:=[]Request{};if status!="connected"{return list}
 a.mu.Lock();defer a.mu.Unlock()
 for _,r:=range a.requests{if r.GithubID==id{list=append(list,r)}}
 return list
}
func (a *App) state()any{
 g:=a.gh.View()
 return map[string]any{"app":"identity","publisher":publisher,"version":appVersion,
  "publisher_evidence":"label embedded in experimental build, not signed release verification",
  "github":g,"catalog":catalog,"requests":a.currentRequests(),
  "delivery":"LOCAL ONLY — request not sent to Arameair",
  "feature_entitlements":"NONE — no approval service configured",
  "can_install":false,
  "status":"RUNNING","run_id":a.runID,"pid":os.Getpid(),"port":a.port,
  "uptime_s":int(time.Since(a.started).Seconds()),"branch":"experiment/identity-home-gpt6-20261025",
  "revision":a.revision,"github_state":g.Status,"records":a.ops.Records(),
  "active_count":a.ops.ActiveCount(),"system_id":a.ops.SystemID(),
  "processor":a.ops.ProcessorState(),"processor_target":a.ops.Target(),
  "events":a.ops.Recent(120),"log_error":a.ops.LogError()}
}
func sendJSON(w http.ResponseWriter,status int,v any){
 w.Header().Set("Content-Type","application/json");w.WriteHeader(status);_ = json.NewEncoder(w).Encode(v)
}
func (a *App) handle(w http.ResponseWriter,r *http.Request){
 h:=w.Header()
 h.Set("Cache-Control","no-store");h.Set("X-Content-Type-Options","nosniff");h.Set("Referrer-Policy","no-referrer")
 h.Set("X-Frame-Options","DENY")
 h.Set("Content-Security-Policy","default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
 goodHost:="127.0.0.1:"+strconv.Itoa(a.port)
 if r.Host!=goodHost && r.Host!="localhost:"+strconv.Itoa(a.port){http.Error(w,"wrong host",421);return}
 if r.Method=="GET" && r.URL.Path=="/api/health"{
  sendJSON(w,200,map[string]string{"app":"identity","version":appVersion,"protocol":"identity.health/0-experimental","note":"self reported; unverified"});return
 }
 if r.Method=="GET" {
  switch r.URL.Path {
  case "/":a.asset(w,"index.html","text/html; charset=utf-8");return
  case "/app.js":a.asset(w,"app.js","text/javascript; charset=utf-8");return
  case "/app.css":a.asset(w,"app.css","text/css; charset=utf-8");return
  }
 }
 if r.URL.Path=="/api/session"&&r.Method=="POST"{
  if r.Header.Get("Origin")!="http://"+r.Host{sendJSON(w,403,map[string]string{"error":"wrong origin"});return}
  var body map[string]string;_ = json.NewDecoder(io.LimitReader(r.Body,1024)).Decode(&body)
  if subtle.ConstantTimeCompare([]byte(body["key"]),[]byte(a.key))!=1{sendJSON(w,401,map[string]string{"error":"invalid start link"});return}
  http.SetCookie(w,&http.Cookie{Name:"identity_trial",Value:a.cookie,Path:"/",HttpOnly:true,SameSite:http.SameSiteStrictMode})
  a.ops.Event("operator","sign_in",a.runID,"OK")
  sendJSON(w,200,map[string]string{"csrf":a.csrf});return
 }
 c,e:=r.Cookie("identity_trial")
 if e!=nil||subtle.ConstantTimeCompare([]byte(c.Value),[]byte(a.cookie))!=1{sendJSON(w,401,map[string]string{"error":"open Identity from the application"});return}
 if r.URL.Path=="/api/session"&&r.Method=="GET"{sendJSON(w,200,map[string]string{"csrf":a.csrf});return}
 if r.URL.Path=="/api/state"&&r.Method=="GET"{sendJSON(w,200,a.state());return}
 if r.URL.Path=="/api/diagnostics"&&r.Method=="GET"{
  w.Header().Set("Content-Type","text/plain; charset=utf-8")
  io.WriteString(w,a.ops.Diagnostics(a.started,a.runID,a.revision,a.port,a.gh.View(),a.directory));return
 }
 if r.URL.Path=="/api/export-request"&&r.Method=="GET"{
  feature:=r.URL.Query().Get("feature")
  for _,q:=range a.currentRequests(){if q.FeatureID==feature{
   w.Header().Set("Content-Disposition","attachment; filename=identity-access-request.json")
   sendJSON(w,200,map[string]any{"notice":"UNVERIFIED DRAFT — NOT AN ACCESS GRANT","publisher":publisher,"request":q});return
  }}
  sendJSON(w,404,map[string]string{"error":"no saved request"});return
 }
 if r.Method!="POST"{sendJSON(w,405,map[string]string{"error":"method not allowed"});return}
 if r.Header.Get("Origin")!="http://"+r.Host||r.Header.Get("X-Identity-CSRF")!=a.csrf||!strings.HasPrefix(r.Header.Get("Content-Type"),"application/json"){
  sendJSON(w,403,map[string]string{"error":"request must come from the Identity window"});return
 }
 if a.ops.Handle(w,r,a) { return }
 switch r.URL.Path{
 case "/api/config/client-id":
  var body map[string]string
  if e:=json.NewDecoder(io.LimitReader(r.Body,1024)).Decode(&body);e!=nil{sendJSON(w,400,map[string]string{"error":"invalid configuration"});return}
  id:=strings.TrimSpace(body["client_id"])
  if !strings.HasPrefix(id,"Iv") || len(id)<8 || len(id)>100 || strings.ContainsAny(id," \t\r\n/\\") {
   sendJSON(w,400,map[string]string{"error":"Enter the public GitHub App Client ID, beginning with Iv."});return
  }
  a.gh.mu.Lock()
  if a.gh.cfg.ClientID!=""{a.gh.mu.Unlock();sendJSON(w,409,map[string]string{"error":"App already configured; no changes made."});return}
  b,_:=json.Marshal(map[string]string{"client_id":id})
  err:=os.WriteFile(filepath.Join(a.directory,"github-app.json"),b,0600)
  if err==nil{a.gh.cfg.ClientID=id;a.gh.status="signed_out"}
  a.gh.mu.Unlock()
  if err!=nil{sendJSON(w,500,map[string]string{"error":"Could not save app configuration"});return}
  sendJSON(w,200,a.state())
 case "/api/github/start":
  if e:=a.gh.Start();e!=nil{sendJSON(w,502,map[string]string{"error":e.Error()});return}
  sendJSON(w,200,a.state())
 case "/api/github/cancel":a.gh.Cancel();sendJSON(w,200,a.state())
 case "/api/github/disconnect":a.gh.Disconnect();sendJSON(w,200,a.state())
 case "/api/request":
  var body map[string]string;if e:=json.NewDecoder(io.LimitReader(r.Body,1024)).Decode(&body);e!=nil{sendJSON(w,400,map[string]string{"error":"invalid request"});return}
  q,e:=a.request(body["feature"]);if e!=nil{sendJSON(w,409,map[string]string{"error":e.Error()});return}
  sendJSON(w,200,map[string]any{"request":q,"delivery":"SAVED LOCALLY, NOT SENT TO ARAMEAIR"})
 default:sendJSON(w,404,map[string]string{"error":"not found"})
 }
}
func (a *App) asset(w http.ResponseWriter,name,ctype string){
 b,e:=assets.ReadFile("web/"+name);if e!=nil{http.Error(w,"asset missing",500);return}
 w.Header().Set("Content-Type",ctype);w.Write(b)
}
func appID(dir string)string{
 if id:=os.Getenv("IDENTITY_GITHUB_CLIENT_ID");id!=""{return strings.TrimSpace(id)}
 exe,_:=os.Executable()
 paths:=[]string{filepath.Join(filepath.Dir(exe),"github-app.json"),"github-app.json",filepath.Join(dir,"github-app.json")}
 for _,p:=range paths{
  b,e:=os.ReadFile(p);if e!=nil{continue}
  var m map[string]string;if json.Unmarshal(b,&m)==nil&&m["client_id"]!=""{return m["client_id"]}
 }
 return ""
}
func openWindow(link string)error{
 var cmd *exec.Cmd
 switch runtime.GOOS{case "windows":cmd=exec.Command("rundll32","url.dll,FileProtocolHandler",link);case "darwin":cmd=exec.Command("open",link);default:cmd=exec.Command("xdg-open",link)}
 return cmd.Start()
}
func main(){
 home,_:=os.UserHomeDir()
 port:=flag.Int("port",7797,"Prototype A loopback port")
 data:=flag.String("data",filepath.Join(home,".identity","home-gpt6-a"),"isolated private Profile A")
 noBrowser:=flag.Bool("no-browser",false,"print operator link without opening browser")
 flag.Parse()
 if *port<1||*port>65535{fmt.Fprintln(os.Stderr,"invalid port");os.Exit(2)}
 // Real GitHub only: Prototype A deliberately has no fixture-login switch.
 cfg:=GitHubConfig{ClientID:appID(*data),WebBase:"https://github.com",APIBase:"https://api.github.com"}
 if err:=os.MkdirAll(*data,0700);err!=nil{fmt.Fprintln(os.Stderr,err);os.Exit(1)}
 lockPath:=filepath.Join(*data,"process.lock")
 lock,err:=os.OpenFile(lockPath,os.O_WRONLY|os.O_CREATE|os.O_EXCL,0600)
 if err!=nil{fmt.Fprintln(os.Stderr,"Profile locked; inspect previous process before removing stale lock");os.Exit(1)}
 fmt.Fprintf(lock,"PID %d\n",os.Getpid());lock.Close()
 defer os.Remove(lockPath)
 a,e:=newApp(*port,*data,cfg);if e!=nil{fmt.Fprintln(os.Stderr,e);os.Exit(1)}
 defer func(){a.ops.Event("identity","stopped",a.runID,"OK");a.ops.Close();os.Remove(filepath.Join(*data,"runtime","operator-code"))}()
 ln,e:=net.Listen("tcp","127.0.0.1:"+strconv.Itoa(*port));if e!=nil{fmt.Fprintln(os.Stderr,e);os.Exit(1)}
 server:=&http.Server{Handler:http.HandlerFunc(a.handle),ReadHeaderTimeout:5*time.Second}
 go server.Serve(ln)
 link:=fmt.Sprintf("http://127.0.0.1:%d/",*port)
 fmt.Println("Identity standalone prototype; publisher:",publisher)
 fmt.Println("Local browser:",link)
 fmt.Println("Private operator code path:",filepath.Join(*data,"runtime","operator-code"))
 if cfg.ClientID==""{fmt.Println("Publisher's public Client ID not configured: use github-app.json next to the executable.")}
 if !*noBrowser{if e:=openWindow(link);e!=nil{fmt.Fprintln(os.Stderr,"browser could not open automatically:",e)}}
 go func(){ticker:=time.NewTicker(6*time.Second);defer ticker.Stop()
   lastGitHub:=a.gh.View().Status
   for {select{case <-a.shutdown:return;case <-ticker.C:
     ghStatus:=a.gh.View().Status
     if ghStatus!=lastGitHub {a.ops.Event("github","verified_state_change","",strings.ToUpper(ghStatus));lastGitHub=ghStatus}
     a.ops.CheckProcessor()
   }}
 }()
 a.ops.CheckProcessor()
 stop:=make(chan os.Signal,1);signal.Notify(stop,os.Interrupt,syscall.SIGTERM)
 select {case <-stop:case <-a.shutdown:}
 ctx,cancel:=context.WithTimeout(context.Background(),3*time.Second);defer cancel();server.Shutdown(ctx)
}
