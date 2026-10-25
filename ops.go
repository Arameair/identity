package main

import (
 "context"
 "crypto/rand"
 "encoding/hex"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "net"
 "net/http"
 "net/url"
 "os"
 "path/filepath"
 "regexp"
 "runtime"
 "strconv"
 "strings"
 "sync"
 "time"
)
type IdentityCard struct { ID string; Name string; Kind string; Created time.Time; Archived *time.Time }
type Registry struct { Schema int; SystemID string; Cards []IdentityCard; Processor string }
type Seen struct { Status string; Target string; Checked time.Time; Detail string }
type EventRecord struct { At time.Time; Source string; Action string; ID string; Result string }
type Operations struct { mu sync.Mutex; dir string; registry Registry; seen Seen; events []EventRecord; log *os.File; logError string }
var patterns=[]*regexp.Regexp{
 regexp.MustCompile("(?i)(?:gh[pousr]_|github_pat_)[A-Za-z0-9_]{5,}"),
 regexp.MustCompile("(?i)Bearer[[:space:]]+[^[:space:],;]+"),
 regexp.MustCompile("(?i)(Authorization|Cookie|Set-Cookie|device_code|user_code|access_token|client_secret|operator.code)[[:space:]]*[:=][[:space:]]*[^[:space:],;]+"),
 regexp.MustCompile("(?i)[?#&](code|key|token|device_code|user_code)=[^[:space:]&#]+"),
 regexp.MustCompile("[A-Z0-9]{4}-[A-Z0-9]{4}"),
 regexp.MustCompile("(?i)[a-f0-9]{32,}"),
}
func Sanitize(s string) string {for _,p:=range patterns{s=p.ReplaceAllString(s,"[REDACTED]")};return s}
func randomRecordID() string {b:=make([]byte,16);if _,e:=rand.Read(b);e!=nil{panic(e)};return hex.EncodeToString(b)}
func saveRegistry(p string,v Registry) error {b,e:=json.MarshalIndent(v,"","  ");if e!=nil{return e};if e=os.WriteFile(p+".new",b,0600);e!=nil{return e};return os.Rename(p+".new",p)}
func OpenOperations(dir string)(*Operations,error){
 o:=&Operations{dir:dir,seen:Seen{Status:"NOT_CONFIGURED",Detail:"No Processor address configured"}}
 p:=filepath.Join(dir,"registry.json");b,e:=os.ReadFile(p)
 if errors.Is(e,os.ErrNotExist){o.registry=Registry{Schema:1,SystemID:"id_"+randomRecordID(),Cards:[]IdentityCard{}};if e=saveRegistry(p,o.registry);e!=nil{return nil,e}}
 else if e!=nil{return nil,e}else if e=json.Unmarshal(b,&o.registry);e!=nil{return nil,fmt.Errorf("unreadable registry left unchanged: %w",e)}
 if o.registry.Schema!=1||len(o.registry.SystemID)!=35{return nil,errors.New("invalid registry schema/system ID")}
 ids:=map[string]bool{};for _,c:=range o.registry.Cards{if c.ID==""||ids[c.ID]{return nil,errors.New("duplicate ID; registry left unchanged")};ids[c.ID]=true}
 for _,f:=range []string{"events.jsonl.2","events.jsonl.1","events.jsonl"}{b,_:=os.ReadFile(filepath.Join(dir,f));for _,l:=range strings.Split(string(b),"\n"){var ev EventRecord;if json.Unmarshal([]byte(l),&ev)==nil&&!ev.At.IsZero(){o.events=append(o.events,ev)}}}
 if len(o.events)>180{o.events=o.events[len(o.events)-180:]}
 o.log,e=os.OpenFile(filepath.Join(dir,"events.jsonl"),os.O_APPEND|os.O_CREATE|os.O_WRONLY,0600);if e!=nil{return nil,e}
 return o,nil
}
func(o *Operations) Event(src,action,id,result string){o.mu.Lock();defer o.mu.Unlock();e:=EventRecord{At:time.Now().UTC(),Source:Sanitize(src),Action:Sanitize(action),ID:Sanitize(id),Result:Sanitize(result)};b,_:=json.Marshal(e);if _,err:=o.log.Write(append(b,'\n'));err!=nil{o.logError="ERROR: event write failed"};o.events=append(o.events,e);if len(o.events)>180{o.events=o.events[len(o.events)-180:]};if st,err:=o.log.Stat();err==nil&&st.Size()>256*1024{o.log.Close();p:=filepath.Join(o.dir,"events.jsonl");os.Remove(p+".2");os.Rename(p+".1",p+".2");os.Rename(p,p+".1");o.log,err=os.OpenFile(p,os.O_APPEND|os.O_CREATE|os.O_WRONLY,0600);if err!=nil{o.logError="ERROR: rotation failed"}}}
func(o *Operations) Close(){o.mu.Lock();defer o.mu.Unlock();if o.log!=nil{o.log.Close()}}
func(o *Operations) LogError()string{o.mu.Lock();defer o.mu.Unlock();if o.logError==""{return "NONE"};return o.logError}
func(o *Operations) Recent(n int)[]EventRecord{o.mu.Lock();defer o.mu.Unlock();if n>len(o.events){n=len(o.events)};a:=make([]EventRecord,n);copy(a,o.events[len(o.events)-n:]);return a}
func(o *Operations) Records()[]IdentityCard{o.mu.Lock();defer o.mu.Unlock();return append([]IdentityCard{},o.registry.Cards...)}
func(o *Operations) SystemID()string{o.mu.Lock();defer o.mu.Unlock();return o.registry.SystemID}
func(o *Operations) ActiveCount()int{o.mu.Lock();defer o.mu.Unlock();n:=0;for _,c:=range o.registry.Cards{if c.Archived==nil{n++}};return n}
func(o *Operations) Target()string{o.mu.Lock();defer o.mu.Unlock();return o.registry.Processor}
func(o *Operations) ProcessorState()Seen{o.mu.Lock();defer o.mu.Unlock();v:=o.seen;if v.Status=="DISCOVERED_UNVERIFIED"&&time.Since(v.Checked)>15*time.Second{v.Status="STALE"};return v}
func validProcessor(raw string,self int)(string,error){if len(raw)>90||raw==""{return "",errors.New("empty/long address")};if !strings.Contains(raw,"://"){raw="http://"+raw};u,e:=url.Parse(raw);if e!=nil||u.Scheme!="http"||u.User!=nil||u.RawQuery!=""||u.Fragment!=""||(u.Path!=""&&u.Path!="/"){return "",errors.New("host and port only")};if u.Hostname()!="localhost"&&u.Hostname()!="127.0.0.1"&&u.Hostname()!="::1"{return "",errors.New("remote host refused")};n,e:=strconv.Atoi(u.Port());if e!=nil||n<1||n>65535||n==self{return "",errors.New("invalid or self port")};return "http://"+net.JoinHostPort("127.0.0.1",strconv.Itoa(n)),nil}
func(o *Operations) CheckProcessor(){target:=o.Target();v:=Seen{Status:"NOT_CONFIGURED",Detail:"Choose Processor loopback port in Setup"};if target!=""{v.Target=target;v.Checked=time.Now().UTC();v.Status="NOT_RUNNING";v.Detail="Public health unreachable; no authenticated integration";cli:=&http.Client{Timeout:1600*time.Millisecond,Transport:&http.Transport{Proxy:nil,DisableKeepAlives:true},CheckRedirect:func(*http.Request,[]*http.Request)error{return http.ErrUseLastResponse}};ctx,cancel:=context.WithTimeout(context.Background(),2*time.Second);defer cancel();req,_:=http.NewRequestWithContext(ctx,"GET",target+"/api/health",nil);resp,e:=cli.Do(req);if e==nil{defer resp.Body.Close();v.Status="WRONG_SERVICE";v.Detail="HTTP public health does not identify expected protocol";if resp.StatusCode==200{var m map[string]any;if json.NewDecoder(io.LimitReader(resp.Body,4096)).Decode(&m)==nil&&m["app"]=="thought-processor"{if p,ok:=m["protocol"].(string);ok&&strings.HasPrefix(p,"thought-processor.diagnostic/"){v.Status="DISCOVERED_UNVERIFIED";v.Detail="Self-reported Processor; not authenticated and NO Engine grant"}}}}}}
 o.mu.Lock();changed:=o.seen.Status!=v.Status||o.seen.Target!=v.Target;o.seen=v;o.mu.Unlock();if changed{o.Event("discovery","probe",v.Target,v.Status)}
}
func(o *Operations) Diagnostics(start time.Time,run,rev string,port int,g GitHubView,profile string)string{p:=o.ProcessorState();status:=g.Status;if status=="connected"{status="AUTHENTICATED via GitHub /user; not authorized for features"};var b strings.Builder;fmt.Fprintf(&b,"Identity Prototype A — Copy Diagnostics\nUTC: %s\nLocal: %s\nSource: Arameair/identity experiment/identity-home-gpt6-20261025\nCommit: %s (UNKNOWN if unverified)\nRuntime: %s/%s PID=%d Port=%d Uptime=%s\nRun ID: %s\nProfile: private local path withheld\nIdentity state: RUNNING\nSystem ID: %s\nActive entities: %d\nGitHub: %s\nProcessor: %s at %s (checked %s); %s\nEvent storage: %s\nNOT IMPLEMENTED: publisher grants, Engine authorization, signed installs, remote access.\nReproduce: independently run Processor, set loopback address in Setup, click Probe.\nRecent events:\n",time.Now().UTC().Format(time.RFC3339),time.Now().Format(time.RFC3339),rev,runtime.GOOS,runtime.GOARCH,os.Getpid(),port,time.Since(start).Truncate(time.Second),run,o.SystemID(),o.ActiveCount(),status,p.Status,p.Target,p.Checked.Format(time.RFC3339),p.Detail,o.LogError());for _,e:=range o.Recent(40){fmt.Fprintf(&b,"%s %s %s %s %s\n",e.At.Format(time.RFC3339),e.Source,e.Action,e.ID,e.Result)};return Sanitize(b.String())}
func(o *Operations) Handle(w http.ResponseWriter,r *http.Request,a *App)bool{
 switch r.URL.Path{
 case "/api/probe":o.CheckProcessor();sendJSON(w,200,o.ProcessorState());return true
 case "/api/processor":
  var x struct{Address string};if json.NewDecoder(io.LimitReader(r.Body,1024)).Decode(&x)!=nil{sendJSON(w,400,map[string]string{"error":"invalid JSON"});return true};if x.Address!=""{target,e:=validProcessor(x.Address,a.port);if e!=nil{sendJSON(w,400,map[string]string{"error":e.Error()});return true};x.Address=target}
  o.mu.Lock();next:=o.registry;next.Processor=x.Address;e:=saveRegistry(filepath.Join(o.dir,"registry.json"),next);if e==nil{o.registry=next};o.mu.Unlock();if e!=nil{sendJSON(w,500,map[string]string{"error":"disk write failed"});return true};o.Event("settings","processor_saved","","OK");o.CheckProcessor();sendJSON(w,200,o.ProcessorState());return true
 case "/api/cards":
  var x struct{Action,ID,Name,Kind string};if json.NewDecoder(io.LimitReader(r.Body,2048)).Decode(&x)!=nil{sendJSON(w,400,map[string]string{"error":"bad JSON"});return true};o.mu.Lock();next:=o.registry
  switch x.Action{case "create":x.Name=strings.TrimSpace(x.Name);if len(x.Name)<1||len(x.Name)>80||strings.ContainsAny(x.Name,"\r\n\x00")||x.Kind==""||len(x.Kind)>35{o.mu.Unlock();sendJSON(w,400,map[string]string{"error":"invalid card"});return true};next.Cards=append(next.Cards,IdentityCard{ID:"ent_"+randomRecordID(),Name:x.Name,Kind:x.Kind,Created:time.Now().UTC()})
  case "archive","restore":found:=false;for i:=range next.Cards{if next.Cards[i].ID==x.ID{found=true;if x.Action=="archive"{t:=time.Now().UTC();next.Cards[i].Archived=&t}else{next.Cards[i].Archived=nil};break}};if !found{o.mu.Unlock();sendJSON(w,404,map[string]string{"error":"card missing"});return true}
  default:o.mu.Unlock();sendJSON(w,400,map[string]string{"error":"unsupported action"});return true}
  e:=saveRegistry(filepath.Join(o.dir,"registry.json"),next);if e==nil{o.registry=next};o.mu.Unlock();if e!=nil{sendJSON(w,500,map[string]string{"error":"disk write failed"});return true};o.Event("records",x.Action,x.ID,"OK");sendJSON(w,200,o.Records());return true
 case "/api/stop":sendJSON(w,202,map[string]string{"status":"STOPPING"});select{case a.shutdown<-struct{}{}:default:};return true
 }
 return false
}
