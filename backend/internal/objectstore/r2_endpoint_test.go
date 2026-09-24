package objectstore

import (
	"context"
	"net/http"
	"testing"
)

func TestBucketScopedEndpointBuildsExpectedObjectPath(t *testing.T){
	s,err:=NewR2Store(R2Config{
		Endpoint:"https://dbc116a454dda65b1ab21ad7744e9473.r2.cloudflarestorage.com/myankafe-finance",
		Region:"auto",
		AccessKey:"test-access",
		SecretKey:"test-secret",
	})
	if err!=nil{t.Fatal(err)}
	if !s.Configured(){t.Fatal("bucket-scoped endpoint should be configured")}
	req,err:=s.newSignedRequest(context.Background(),http.MethodGet,"transactions/ENTITY/TX/A/file.pdf","",nil)
	if err!=nil{t.Fatal(err)}
	want:="/myankafe-finance/transactions/ENTITY/TX/A/file.pdf"
	if req.URL.Path!=want{t.Fatalf("path=%q want %q",req.URL.Path,want)}
	if req.Header.Get("Authorization")==""{t.Fatal("missing SigV4 authorization header")}
}
