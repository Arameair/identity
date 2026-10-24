package main

import (
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"
)

func TestLocalRequestPersistsWithoutIssuingGrant(t *testing.T){
 dir:=t.TempDir()
 cfg:=GitHubConfig{ClientID:"IvEXAMPLE12345",WebBase:"https://github.com",APIBase:"https://api.github.com"}
 a,e:=newApp(7790,dir,cfg);if e!=nil{t.Fatal(e)}
 if _,e=a.request("thought-processor");e==nil{t.Fatal("request accepted before GitHub sign-in")}
 a.gh.mu.Lock();a.gh.status="connected";a.gh.userID=42;a.gh.login="requester";a.gh.mu.Unlock()
 r,e:=a.request("thought-processor");if e!=nil{t.Fatal(e)}
 if r.Status!="SAVED LOCALLY — NOT SUBMITTED"{t.Fatalf("misleading status: %s",r.Status)}
 repeat,e:=a.request("thought-processor")
 if e!=nil||repeat.ID!=r.ID{t.Fatal("double request created another record")}
 if _,e=a.request("unknown");e==nil{t.Fatal("unknown feature allowed")}
 b,e:=newApp(7790,dir,cfg);if e!=nil{t.Fatal(e)}
 b.gh.mu.Lock();b.gh.status="connected";b.gh.userID=777;b.gh.login="other";b.gh.mu.Unlock()
 if len(b.currentRequests())!=0{t.Fatal("another account saw draft")}
 b.gh.mu.Lock();b.gh.userID=42;b.gh.login="requester";b.gh.mu.Unlock()
 if len(b.currentRequests())!=1{t.Fatal("draft did not survive app restart")}
}

func TestSessionCSRFOriginAndClientID(t *testing.T){
 dir:=t.TempDir()
 cfg:=GitHubConfig{WebBase:"https://github.com",APIBase:"https://api.github.com"}
 a,e:=newApp(7790,dir,cfg);if e!=nil{t.Fatal(e)}
 run:=func(method,path,body,origin,cookie,csrf string)*httptest.ResponseRecorder{
  req:=httptest.NewRequest(method,"http://127.0.0.1:7790"+path,strings.NewReader(body))
  req.Host="127.0.0.1:7790";req.Header.Set("Content-Type","application/json")
  if origin!=""{req.Header.Set("Origin",origin)}
  if csrf!=""{req.Header.Set("X-Identity-CSRF",csrf)}
  if cookie!=""{req.AddCookie(&http.Cookie{Name:"identity_trial",Value:cookie})}
  w:=httptest.NewRecorder();a.handle(w,req);return w
 }
 if run("GET","/api/state","","","","").Code!=401{t.Fatal("unauthenticated app state")}
 if run("POST","/api/session",`{"key":"wrong"}`,"http://127.0.0.1:7790","","").Code!=401{t.Fatal("invalid bootstrap")}
 signed:=run("POST","/api/session",`{"key":"`+a.key+`"}`,"http://127.0.0.1:7790","","")
 if signed.Code!=200{t.Fatal(signed.Body.String())}
 var reply map[string]string
 if e:=json.Unmarshal(signed.Body.Bytes(),&reply);e!=nil{t.Fatal(e)}
 csrf:=reply["csrf"];cookie:=signed.Result().Cookies()[0].Value
 if run("POST","/api/config/client-id",`{"client_id":"IvGOOD123456"}`,"http://evil.example",cookie,csrf).Code!=403{t.Fatal("origin bypass")}
 if run("POST","/api/config/client-id",`{"client_id":"IvGOOD123456"}`,"http://127.0.0.1:7790",cookie,"wrong").Code!=403{t.Fatal("csrf bypass")}
 saved:=run("POST","/api/config/client-id",`{"client_id":"IvGOOD123456"}`,"http://127.0.0.1:7790",cookie,csrf)
 if saved.Code!=200{t.Fatal(saved.Body.String())}
 if appID(dir)!="IvGOOD123456"{t.Fatal("Client ID not persisted")}
 if run("POST","/api/request",`{"feature":"thought-processor"}`,"http://127.0.0.1:7790",cookie,csrf).Code!=409{t.Fatal("request accepted before auth")}
 if run("GET","/api/export-request?feature=thought-processor","","","","").Code!=401{t.Fatal("export is not session-bound")}
 if run("GET","/api/session","","",cookie,"").Code!=200{t.Fatal("session csrf restore failed")}
}
