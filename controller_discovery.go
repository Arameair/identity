package main

// Discover a separately started Thought Controller on loopback.
// No process launching, and no GitHub writes.
import (
 "context"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "net"
 "net/http"
 "os"
 "path/filepath"
 "sync"
 "time"
)

const defaultThoughtControllerPort = 7797
type ControllerDiscovery struct {
 mu sync.Mutex
 dataDir string
 port int
}
type controllerConfig struct { Port int `json:"port"` }
type ControllerResult struct {
 Port int `json:"port"`
 State string `json:"state"`
 Detail string `json:"detail"`
 Recognized bool `json:"recognized"`
 URL string `json:"url,omitempty"`
 Worker string `json:"worker,omitempty"`
 Simulated bool `json:"simulated"`
 Protocol string `json:"protocol,omitempty"`
}
type controllerHealth struct {
 App string `json:"app"`
 Service string `json:"service"`
 Protocol string `json:"protocol"`
 Simulated bool `json:"simulated"`
 Worker struct {State string `json:"state"`} `json:"worker"`
}
func validControllerPort(port int)bool {return port>=1024&&port<=65535}
func openControllerDiscovery(dataDir string)(*ControllerDiscovery,error){
 d:=&ControllerDiscovery{dataDir:dataDir,port:defaultThoughtControllerPort}
 b,e:=os.ReadFile(filepath.Join(dataDir,"controller-discovery.json"))
 if errors.Is(e,os.ErrNotExist){return d,nil}
 if e!=nil{return nil,fmt.Errorf("controller settings could not be read: %w",e)}
 var c controllerConfig
 if e=json.Unmarshal(b,&c);e!=nil{return nil,fmt.Errorf("controller settings invalid; file left untouched: %w",e)}
 if !validControllerPort(c.Port){return nil,errors.New("controller settings contain invalid port; file left untouched")}
 d.port=c.Port;return d,nil
}
func(d *ControllerDiscovery)currentPort()int{d.mu.Lock();defer d.mu.Unlock();return d.port}
func(d *ControllerDiscovery)SetPort(port int)error{
 if !validControllerPort(port){return errors.New("port must be between 1024 and 65535")}
 d.mu.Lock();defer d.mu.Unlock()
 b,e:=json.MarshalIndent(controllerConfig{Port:port},"","  ");if e!=nil{return e}
 f,e:=os.CreateTemp(d.dataDir,".controller-discovery-*");if e!=nil{return e}
 tmp:=f.Name();defer os.Remove(tmp)
 if e=f.Chmod(0600);e!=nil{f.Close();return e}
 if _,e=f.Write(append(b,'\n'));e!=nil{f.Close();return e}
 if e=f.Sync();e!=nil{f.Close();return e}
 if e=f.Close();e!=nil{return e}
 if e=os.Rename(tmp,filepath.Join(d.dataDir,"controller-discovery.json"));e!=nil{return e}
 d.port=port;return nil
}
// An open port is insufficient. Require Thought Controller's own health ID.
func probePort(ctx context.Context,port int)ControllerResult{
 r:=ControllerResult{Port:port,State:"NOT RUNNING",Detail:"Thought Controller wasn't detected on this port. Start it in Project Controller, then check again."}
 if !validControllerPort(port){r.State="INVALID PORT";r.Detail="Port must be 1024–65535";return r}
 address:=fmt.Sprintf("127.0.0.1:%d",port)
 conn,e:=net.DialTimeout("tcp",address,180*time.Millisecond);if e!=nil{return r}
 _=conn.Close()
 r.State="OTHER SERVICE OR NOT READY"
 r.Detail="A program is listening on this port, but it hasn't identified itself as Thought Controller."
 client:=&http.Client{Timeout:450*time.Millisecond,CheckRedirect:func(*http.Request,[]*http.Request)error{return http.ErrUseLastResponse}}
 for _,endpoint:=range []string{"/api/health","/api/heartbeat"}{
  req,e:=http.NewRequestWithContext(ctx,http.MethodGet,"http://"+address+endpoint,nil);if e!=nil{return r}
  response,e:=client.Do(req);if e!=nil{continue}
  if response.StatusCode!=http.StatusOK{response.Body.Close();continue}
  var health controllerHealth
  e=json.NewDecoder(io.LimitReader(response.Body,4096)).Decode(&health);_ =response.Body.Close()
  if e!=nil{continue}
  if endpoint=="/api/health"&&health.App=="thought-controller"||endpoint=="/api/heartbeat"&&health.Service=="thought-controller"{
   r.State="DETECTED"
   r.Detail="Thought Controller is responding on this computer. Open its interface below. This is a read-only health check, not an authenticated Identity registration."
   r.URL="http://"+address+"/";r.Recognized=true;r.Simulated=health.Simulated;r.Protocol=health.Protocol;r.Worker=health.Worker.State
   return r
  }
  return r // another service answered; never trust it as Thought Controller
 }
 return r
}
func(d *ControllerDiscovery)Check()ControllerResult{
 ctx,cancel:=context.WithTimeout(context.Background(),1300*time.Millisecond);defer cancel()
 return probePort(ctx,d.currentPort())
}
func scanControllerPorts(ctx context.Context,start,end int)[]ControllerResult{
 if start<1024||end>65535||start>end||end-start>160{return nil}
 type result struct{port int;r ControllerResult}
 jobs:=make(chan int);hits:=make(chan result,end-start+1)
 var wg sync.WaitGroup
 for i:=0;i<16;i++{wg.Add(1);go func(){defer wg.Done();for p:=range jobs{r:=probePort(ctx,p);if r.Recognized{hits<-result{p,r}}}}()}
 go func(){defer close(jobs);for p:=start;p<=end;p++{select{case <-ctx.Done():return;case jobs<-p:}}}()
 wg.Wait();close(hits)
 results:=make([]ControllerResult,0);byPort:=map[int]ControllerResult{}
 for hit:=range hits{byPort[hit.port]=hit.r}
 for p:=start;p<=end;p++{if r,ok:=byPort[p];ok{results=append(results,r)}}
 return results
}
func(d *ControllerDiscovery)Scan()[]ControllerResult{
 ctx,cancel:=context.WithTimeout(context.Background(),4500*time.Millisecond);defer cancel()
 return scanControllerPorts(ctx,7778,7899)
}
