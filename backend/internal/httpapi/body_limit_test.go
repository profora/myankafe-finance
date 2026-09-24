package httpapi

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBodyLimitCapsJSONMutations(t *testing.T){
	s:=&Server{}
	h:=s.bodyLimit(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
		_,err:=io.ReadAll(r.Body)
		if err!=nil{
			http.Error(w,err.Error(),http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	req:=httptest.NewRequest(http.MethodPost,"/api/v1/test",bytes.NewBuffer(make([]byte,maxJSONRequestBytes+1)))
	req.Header.Set("Content-Type","application/json")
	rec:=httptest.NewRecorder()
	h.ServeHTTP(rec,req)
	if rec.Code!=http.StatusRequestEntityTooLarge{
		t.Fatalf("status=%d want %d body=%s",rec.Code,http.StatusRequestEntityTooLarge,rec.Body.String())
	}
}

func TestBodyLimitLeavesMultipartToUploadHandler(t *testing.T){
	s:=&Server{}
	h:=s.bodyLimit(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
		data,err:=io.ReadAll(r.Body);if err!=nil{t.Fatal(err)}
		if len(data)!=int(maxJSONRequestBytes+1){t.Fatalf("bytes=%d",len(data))}
		w.WriteHeader(http.StatusNoContent)
	}))
	req:=httptest.NewRequest(http.MethodPost,"/api/v1/upload",strings.NewReader(strings.Repeat("x",int(maxJSONRequestBytes+1))))
	req.Header.Set("Content-Type","multipart/form-data; boundary=test")
	rec:=httptest.NewRecorder()
	h.ServeHTTP(rec,req)
	if rec.Code!=http.StatusNoContent{t.Fatalf("status=%d",rec.Code)}
}
