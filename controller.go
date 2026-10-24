package main

// One explicit SHA-256-pinned local Thought Controller executable.
// Identity never downloads or builds it and never takes over an external process.
import (
 "context"
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "net"
 "net/http"
 "os"
 "os/exec"
 "path/filepath"
 "runtime"
 "strconv"
 "strings"
 "sync"
 "syscall"
 "time"
)

type ControllerApproval struct {
 Path string `json:"path"`
 SHA256 string `json:"sha256"`
 Port int `json:"port"`
 AutoStart bool `json:"auto_start"`
 ApprovedAt time.Time `json:"approved_at"`
}
type ControllerStatus struct {
 State string `json:"state"`
 Detail string `json:"detail"`
 URL string `json:"url,omitempty"`
 Path string `json:"path,omitempty"`
 SHA256 string `json:"sha256,omitempty"`
 Port int `json:"port,omitempty"`
 AutoStart bool `json:"auto_start"`
 Owned bool `json:"owned"`
 PID int `json:"pid,omitempty"`
 Worker string `json:"worker,omitempty"`
 Simulated bool `json:"simulated"`
}
type tcProbe struct {
 App string `json:"app"`
 Service string `json:"service"`
 Simulated bool `json:"simulated"`
 Worker struct{State string `json:"state"`} `json:"worker"`
}
type ControllerManager struct {
 mu sync.Mutex
 dir string
 approved *ControllerApproval
 cmd *exec.Cmd
 done chan error
 pid int
 lastError string
}

func openControllerManager(dir string) (*ControllerManager,error) {
 m:=&ControllerManager{dir:dir}
 b,err:=os.ReadFile(filepath.Join(dir,"controller-approval.json"))
 if errors.Is(err,os.ErrNotExist){return m,nil}
 if err!=nil{return nil,err}
 var a ControllerApproval
 if err=json.Unmarshal(b,&a);err!=nil{return nil,fmt.Errorf("unreadable controller approval, file unchanged: %w",err)}
 if len(a.SHA256)!=64||a.Port<1024||a.Port>65535||!filepath.IsAbs(a.Path){return nil,errors.New("invalid stored controller approval; file unchanged")}
 m.approved=&a
 return m,nil
}
func executableDigest(path string)(string,error){
 f,err:=os.Open(path);if err!=nil{return "",err};defer f.Close()
 info,err:=f.Stat();if err!=nil{return "",err}
 if !info.Mode().IsRegular(){return "",errors.New("expected a regular executable file")}
 if runtime.GOOS=="windows"&&!strings.EqualFold(filepath.Ext(path),".exe"){return "",errors.New("choose a .exe on Windows")}
 if runtime.GOOS!="windows"&&info.Mode()&0111==0{return "",errors.New("executable permission is missing")}
 h:=sha256.New();if _,err=io.Copy(h,f);err!=nil{return "",err}
 return hex.EncodeToString(h.Sum(nil)),nil
}
func (m *ControllerManager) approval()*ControllerApproval{
 m.mu.Lock();defer m.mu.Unlock()
 if m.approved==nil{return nil};a:=*m.approved;return &a
}
func (m *ControllerManager) Approve(path string,port int,auto bool)(ControllerStatus,error){
 if port<1024||port>65535{return ControllerStatus{},errors.New("port must be 1024–65535")}
 if !filepath.IsAbs(path){return ControllerStatus{},errors.New("enter an absolute path to the trusted Thought Controller executable")}
 abs,err:=filepath.Abs(strings.TrimSpace(path));if err!=nil{return ControllerStatus{},err}
 digest,err:=executableDigest(abs);if err!=nil{return ControllerStatus{},err}
 m.mu.Lock();defer m.mu.Unlock();m.reapLocked()
 if m.cmd!=nil{return ControllerStatus{},errors.New("stop Identity-owned controller before changing its approval")}
 a:=ControllerApproval{Path:abs,SHA256:digest,Port:port,AutoStart:auto,ApprovedAt:time.Now().UTC()}
 b,err:=json.MarshalIndent(a,"","  ");if err!=nil{return ControllerStatus{},err}
 f,err:=os.CreateTemp(m.dir,".controller-approval-*");if err!=nil{return ControllerStatus{},err}
 tmp:=f.Name();defer os.Remove(tmp)
 if err=f.Chmod(0600);err!=nil{f.Close();return ControllerStatus{},err}
 if _,err=f.Write(append(b,'\n'));err!=nil{f.Close();return ControllerStatus{},err}
 if err=f.Sync();err!=nil{f.Close();return ControllerStatus{},err}
 if err=f.Close();err!=nil{return ControllerStatus{},err}
 if err=os.Rename(tmp,filepath.Join(m.dir,"controller-approval.json"));err!=nil{return ControllerStatus{},err}
 m.approved=&a;m.lastError=""
 return m.statusLocked(),nil
}
func (m *ControllerManager) Revoke()(ControllerStatus,error){
 m.mu.Lock();defer m.mu.Unlock();m.reapLocked()
 if m.cmd!=nil{return ControllerStatus{},errors.New("stop the owned controller before revoking approval")}
 if err:=os.Remove(filepath.Join(m.dir,"controller-approval.json"));err!=nil&&!errors.Is(err,os.ErrNotExist){return ControllerStatus{},err}
 m.approved=nil;m.lastError=""
 return m.statusLocked(),nil
}
func (m *ControllerManager) reapLocked(){
 if m.cmd==nil{return}
 select{
 case err:=<-m.done:
  m.lastError="controller process exited"
  if err!=nil{m.lastError="controller process exited: "+err.Error()}
  m.cmd=nil;m.done=nil;m.pid=0
 default:
 }
}
func loopbackProbe(port int)(tcProbe,error){
 var p tcProbe
 client:=&http.Client{Timeout:650*time.Millisecond,CheckRedirect:func(*http.Request,[]*http.Request)error{return http.ErrUseLastResponse}}
 ctx,cancel:=context.WithTimeout(context.Background(),650*time.Millisecond);defer cancel()
 base:=fmt.Sprintf("http://127.0.0.1:%d",port)
 for _,route:=range []string{"/api/health","/api/heartbeat"}{
  req,err:=http.NewRequestWithContext(ctx,http.MethodGet,base+route,nil);if err!=nil{return p,err}
  res,err:=client.Do(req);if err!=nil{return p,err}
  if res.StatusCode!=200{res.Body.Close();continue}
  err=json.NewDecoder(io.LimitReader(res.Body,8192)).Decode(&p);res.Body.Close()
  if err!=nil{return p,err}
  if route=="/api/health"&&p.App=="thought-controller"{return p,nil}
  if route=="/api/heartbeat"&&p.Service=="thought-controller"{p.Worker.State="CHECK IN CONTROLLER";return p,nil}
  return p,errors.New("unexpected application responded on configured port")
 }
 return p,errors.New("no compatible controller health endpoint")
}
func portOccupied(port int)bool{
 c,err:=net.DialTimeout("tcp",fmt.Sprintf("127.0.0.1:%d",port),220*time.Millisecond)
 if err!=nil{return false};c.Close();return true
}
func (m *ControllerManager) statusLocked()ControllerStatus{
 m.reapLocked()
 if m.approved==nil{return ControllerStatus{State:"NOT CONFIGURED",Detail:"Choose and approve a local controller executable once."}}
 a:=m.approved
 s:=ControllerStatus{Path:a.Path,SHA256:a.SHA256,Port:a.Port,AutoStart:a.AutoStart,URL:fmt.Sprintf("http://127.0.0.1:%d/",a.Port)}
 digest,err:=executableDigest(a.Path)
 if err!=nil{s.State="EXECUTABLE MISSING";s.Detail=err.Error();return s}
 if digest!=a.SHA256{s.State="BUILD CHANGED";s.Detail="Approved binary changed. New explicit approval is required.";return s}
 if m.cmd!=nil{
  s.Owned=true;s.PID=m.pid
  p,err:=loopbackProbe(a.Port)
  if err==nil{s.State="RUNNING";s.Detail="Identity owns this running process; thought/worker readiness is separate.";s.Worker=p.Worker.State;s.Simulated=p.Simulated;return s}
  s.State="STARTING";s.Detail="Started, but health not yet confirmed: "+err.Error();return s
 }
 if portOccupied(a.Port){
  p,err:=loopbackProbe(a.Port)
  s.State="EXTERNAL INSTANCE";s.Detail="Port occupied. Identity will not take over or stop another process."
  if err==nil{s.Worker=p.Worker.State;s.Simulated=p.Simulated}
  return s
 }
 s.State="STOPPED";s.Detail="Approved controller is not running."
 if m.lastError!=""{s.Detail+=" Last result: "+m.lastError}
 return s
}
func (m *ControllerManager) Status()ControllerStatus{m.mu.Lock();defer m.mu.Unlock();return m.statusLocked()}
func controllerEnv(port int)[]string{
 env:=[]string{}
 for _,v:=range os.Environ(){
  key,_,_:=strings.Cut(v,"=");u:=strings.ToUpper(key)
  if u=="PORT"||strings.HasPrefix(u,"PC_")||u=="IDENTITY_DATA"||u=="IDENTITY_FIRST_ADMIN_DATA"||u=="GH_TOKEN"||u=="GITHUB_TOKEN"||u=="GH_ENTERPRISE_TOKEN"||u=="GITHUB_ENTERPRISE_TOKEN"{continue}
  env=append(env,v)
 }
 return append(env,"PORT="+strconv.Itoa(port))
}
func (m *ControllerManager) Start()(ControllerStatus,error){
 m.mu.Lock();defer m.mu.Unlock();m.reapLocked()
 if m.approved==nil{return m.statusLocked(),errors.New("approve an exact executable first")}
 a:=m.approved
 digest,err:=executableDigest(a.Path)
 if err!=nil{return m.statusLocked(),err}
 if digest!=a.SHA256{return m.statusLocked(),errors.New("binary changed; reapprove before starting")}
 if m.cmd!=nil{return m.statusLocked(),nil}
 if portOccupied(a.Port){return m.statusLocked(),errors.New("configured port occupied; refusing to start or adopt external process")}
 dataDir:=filepath.Join(m.dir,"thought-controller-managed")
 if err=os.MkdirAll(dataDir,0700);err!=nil{return m.statusLocked(),err}
 cmd:=exec.Command(a.Path,"-data",dataDir)
 cmd.Dir=filepath.Dir(a.Path);cmd.Env=controllerEnv(a.Port)
 logFile,err:=os.OpenFile(filepath.Join(m.dir,"thought-controller-launch.log"),os.O_APPEND|os.O_CREATE|os.O_WRONLY,0600)
 if err!=nil{return m.statusLocked(),err}
 cmd.Stdout=logFile;cmd.Stderr=logFile
 if err=cmd.Start();err!=nil{logFile.Close();m.lastError=err.Error();return m.statusLocked(),err}
 m.cmd=cmd;m.done=make(chan error,1);m.pid=cmd.Process.Pid
 go func(){err:=cmd.Wait();logFile.Close();m.done<-err}()
 m.lastError=""
 deadline:=time.Now().Add(5*time.Second)
 for time.Now().Before(deadline){
  m.reapLocked()
  if m.cmd==nil{return m.statusLocked(),errors.New("controller exited during startup; inspect local launch log")}
  if p,e:=loopbackProbe(a.Port);e==nil&&(p.App=="thought-controller"||p.Service=="thought-controller"){return m.statusLocked(),nil}
  time.Sleep(110*time.Millisecond)
 }
 _=stopOwned(cmd,m.done)
 m.cmd=nil;m.done=nil;m.pid=0
 m.lastError="executable started but compatible health check failed; inspect local launch log"
 return m.statusLocked(),errors.New(m.lastError)
}
func stopOwned(cmd *exec.Cmd,done <-chan error)error{
 if cmd==nil||cmd.Process==nil{return nil}
 if runtime.GOOS=="windows"{_ =cmd.Process.Kill()}else{_ =cmd.Process.Signal(syscall.SIGTERM)}
 select{
 case <-done:return nil
 case <-time.After(3*time.Second):
  _=cmd.Process.Kill()
  select{case <-done:case <-time.After(time.Second):}
  return errors.New("controller required forced shutdown")
 }
}
func (m *ControllerManager) Stop()(ControllerStatus,error){
 m.mu.Lock();defer m.mu.Unlock();m.reapLocked()
 if m.cmd==nil{return m.statusLocked(),errors.New("no Identity-owned process to stop; external controllers are untouched")}
 err:=stopOwned(m.cmd,m.done)
 m.cmd=nil;m.done=nil;m.pid=0
 m.lastError="Identity-owned controller stopped"
 if err!=nil{m.lastError=err.Error()}
 return m.statusLocked(),err
}
func (m *ControllerManager) Close(){
 m.mu.Lock();defer m.mu.Unlock();m.reapLocked()
 if m.cmd!=nil{_ =stopOwned(m.cmd,m.done);m.cmd=nil;m.done=nil;m.pid=0}
}
func (m *ControllerManager) AutoStart(){
 a:=m.approval()
 if a!=nil&&a.AutoStart{if _,err:=m.Start();err!=nil{fmt.Fprintln(os.Stderr,"Thought Controller auto-start:",err)}}
}
