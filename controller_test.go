package main

import (
 "fmt"
 "net"
 "net/http"
 "os"
 "os/exec"
 "path/filepath"
 "runtime"
 "strings"
 "testing"
)

func controllerTestBinary(t *testing.T)string{
 t.Helper()
 d:=t.TempDir()
 src:=`package main
import("encoding/json";"net/http";"os";"os/signal";"syscall")
func main(){m:=http.NewServeMux();m.HandleFunc("/api/health",func(w http.ResponseWriter,r *http.Request){_ = json.NewEncoder(w).Encode(map[string]any{"app":"thought-controller","simulated":false})});s:=&http.Server{Addr:"127.0.0.1:"+os.Getenv("PORT"),Handler:m};go s.ListenAndServe();c:=make(chan os.Signal,1);signal.Notify(c,os.Interrupt,syscall.SIGTERM);<-c;_ = s.Close()}
`
 if err:=os.WriteFile(filepath.Join(d,"helper.go"),[]byte(src),0600);err!=nil{t.Fatal(err)}
 name:="thought-controller";if runtime.GOOS=="windows"{name+=".exe"}
 bin:=filepath.Join(d,name)
 cmd:=exec.Command("go","build","-o",bin,"helper.go");cmd.Dir=d
 if b,e:=cmd.CombinedOutput();e!=nil{t.Fatalf("compile fixture: %v: %s",e,b)}
 return bin
}
func controllerFreePort(t *testing.T)int{
 t.Helper();l,e:=net.Listen("tcp","127.0.0.1:0");if e!=nil{t.Fatal(e)}
 p:=l.Addr().(*net.TCPAddr).Port;l.Close();return p
}
func TestThoughtControllerStartStopAndSavedPermission(t *testing.T){
 a:=testApp(t,nil);defer a.controller.Close()
 path:=controllerTestBinary(t);port:=controllerFreePort(t)
 if _,e:=a.controller.Start();e==nil{t.Fatal("unapproved start succeeded")}
 st,e:=a.controller.Approve(path,port,true);if e!=nil||st.State!="STOPPED"{t.Fatalf("approval: %+v %v",st,e)}
 st,e=a.controller.Start();if e!=nil||st.State!="RUNNING"||!st.Owned||st.PID==0{t.Fatalf("actual startup: %+v %v",st,e)}
 pid:=st.PID
 again,e:=a.controller.Start();if e!=nil||again.PID!=pid{t.Fatal("duplicate process launched")}
 if _,e=a.controller.Revoke();e==nil{t.Fatal("revoked running executable")}
 if _,e=a.controller.Stop();e!=nil{t.Fatal(e)}
 st=a.controller.Status();if st.State!="STOPPED"||st.Owned{t.Fatalf("stop: %+v",st)}
 next,e:=openControllerManager(a.dataDir);if e!=nil{t.Fatal(e)}
 if next.approval()==nil||!next.approval().AutoStart{t.Fatal("lost approval")}
 next.AutoStart();if next.Status().State!="RUNNING"{t.Fatalf("automatic start: %+v",next.Status())}
 next.Close()
 st,e=a.controller.Revoke();if e!=nil||st.State!="NOT CONFIGURED"{t.Fatalf("revoke: %+v %v",st,e)}
}
func TestThoughtControllerRefusesOccupiedPort(t *testing.T){
 a:=testApp(t,nil);defer a.controller.Close()
 l,e:=net.Listen("tcp","127.0.0.1:0");if e!=nil{t.Fatal(e)};defer l.Close()
 port:=l.Addr().(*net.TCPAddr).Port
 _,e=a.controller.Approve(controllerTestBinary(t),port,true);if e!=nil{t.Fatal(e)}
 if _,e=a.controller.Start();e==nil{t.Fatal("took over external listener")}
 if s:=a.controller.Status();s.State!="EXTERNAL INSTANCE"||s.Owned{t.Fatalf("external misclassified: %+v",s)}
 if _,e=a.controller.Stop();e==nil{t.Fatal("stopped external listener")}
}
func TestControllerBuildChangeRequiresReapproval(t *testing.T){
 a:=testApp(t,nil);defer a.controller.Close()
 path:=controllerTestBinary(t)
 if _,e:=a.controller.Approve(path,controllerFreePort(t),true);e!=nil{t.Fatal(e)}
 f,e:=os.OpenFile(path,os.O_WRONLY|os.O_APPEND,0);if e!=nil{t.Fatal(e)}
 if _,e=f.Write([]byte("\nchanged"));e!=nil{t.Fatal(e)};f.Close()
 if s:=a.controller.Status();s.State!="BUILD CHANGED"{t.Fatalf("missed build mismatch: %+v",s)}
 if _,e=a.controller.Start();e==nil{t.Fatal("allowed changed binary")}
}
func TestControllerDoesNotInheritOtherAppOrGitHubTokens(t *testing.T){
 t.Setenv("PC_DATA","private-folder");t.Setenv("PC_BRANCH","experiment/identity")
 t.Setenv("PORT","7796");t.Setenv("GH_TOKEN","secret-test")
 env:=strings.Join(controllerEnv(7797),"\n")
 for _,s:=range []string{"private-folder","experiment/identity","PORT=7796","secret-test"}{
  if strings.Contains(env,s){t.Fatalf("inherited %s",s)}
 }
 if !strings.Contains(env,"PORT=7797"){t.Fatal("wrong port")}
}
func TestExternalControllerRespondingToHealthNotAdopted(t *testing.T){
 a:=testApp(t,nil);defer a.controller.Close()
 l,e:=net.Listen("tcp","127.0.0.1:0");if e!=nil{t.Fatal(e)};defer l.Close()
 srv:=&http.Server{Handler:http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  if r.URL.Path=="/api/health"{fmt.Fprint(w,`{"app":"thought-controller","simulated":true}`)}else{http.NotFound(w,r)}
 })}
 go srv.Serve(l);defer srv.Close()
 p:=l.Addr().(*net.TCPAddr).Port
 _,e=a.controller.Approve(controllerTestBinary(t),p,true);if e!=nil{t.Fatal(e)}
 st:=a.controller.Status()
 if st.State!="EXTERNAL INSTANCE"||st.Owned{t.Fatalf("external process incorrectly owned: %+v",st)}
}
