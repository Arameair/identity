package main

import(
 "context"
 "encoding/json"
 "net"
 "net/http"
 "os"
 "path/filepath"
 "strings"
 "testing"
 "time"
)

func fixtureController(t *testing.T,route,body string)(int,func()){
 t.Helper()
 l,e:=net.Listen("tcp","127.0.0.1:0");if e!=nil{t.Fatal(e)}
 mux:=http.NewServeMux()
 mux.HandleFunc(route,func(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","application/json");_,_=w.Write([]byte(body))})
 srv:=&http.Server{Handler:mux}
 go srv.Serve(l)
 return l.Addr().(*net.TCPAddr).Port,func(){_ =srv.Close()}
}
func TestControllerDetectsActualHealth(t *testing.T){
 p,close:=fixtureController(t,"/api/health",`{"app":"thought-controller","simulated":true,"worker":{"state":"NOT READY"}}`);defer close()
 r:=probePort(context.Background(),p)
 if !r.Recognized||r.State!="DETECTED"||!r.Simulated||r.URL==""||r.Worker!="NOT READY"{t.Fatalf("wrong result: %+v",r)}
}
func TestControllerDetectsOlderHeartbeat(t *testing.T){
 p,close:=fixtureController(t,"/api/heartbeat",`{"service":"thought-controller","ok":true}`);defer close()
 if r:=probePort(context.Background(),p);!r.Recognized{t.Fatalf("not detected: %+v",r)}
}
func TestControllerRejectsOtherPrograms(t *testing.T){
 p,close:=fixtureController(t,"/api/health",`{"app":"identity"}`);defer close()
 r:=probePort(context.Background(),p)
 if r.Recognized||r.URL!=""||r.State!="OTHER SERVICE OR NOT READY"{t.Fatalf("trusted wrong program: %+v",r)}
}
func TestControllerPortSavedButNoProcessLaunched(t *testing.T){
 a:=testApp(t,nil)
 if a.controller.currentPort()!=7797{t.Fatal("wrong default port")}
 l,e:=net.Listen("tcp","127.0.0.1:0");if e!=nil{t.Fatal(e)}
 p:=l.Addr().(*net.TCPAddr).Port;l.Close()
 if e=a.controller.SetPort(p);e!=nil{t.Fatal(e)}
 next,e:=openControllerDiscovery(a.dataDir);if e!=nil{t.Fatal(e)}
 if next.currentPort()!=p{t.Fatal("port not persisted")}
 r:=next.Check();if r.Recognized||r.State!="NOT RUNNING"{t.Fatalf("open port invented: %+v",r)}
 if e=next.SetPort(80);e==nil{t.Fatal("invalid port allowed")}
 bytes,e:=os.ReadFile(filepath.Join(a.dataDir,"controller-discovery.json"));if e!=nil{t.Fatal(e)}
 var value controllerConfig
 if e=json.Unmarshal(bytes,&value);e!=nil||value.Port!=p{t.Fatal("port state corrupted")}
}
func TestControllerScannerOnlyKeepsVerifiedResults(t *testing.T){
 p,close:=fixtureController(t,"/api/health",`{"app":"thought-controller"}`);defer close()
 ctx,cancel:=context.WithTimeout(context.Background(),3*time.Second);defer cancel()
 results:=scanControllerPorts(ctx,p,p)
 if len(results)!=1||results[0].Port!=p{t.Fatalf("scan: %+v",results)}
}
func TestBrokenControllerSettingsNotDiscarded(t *testing.T){
 a:=testApp(t,nil)
 path:=filepath.Join(a.dataDir,"controller-discovery.json")
 if e:=os.WriteFile(path,[]byte("{broken"),0600);e!=nil{t.Fatal(e)}
 if _,e:=openControllerDiscovery(a.dataDir);e==nil{t.Fatal("accepted corrupt settings")}
 b,e:=os.ReadFile(path);if e!=nil||!strings.Contains(string(b),"broken"){t.Fatal("overwrote original file")}
}
