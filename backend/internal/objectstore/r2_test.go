package objectstore

import "testing"

func TestR2DisabledWhenEmpty(t *testing.T){
	s,err:=NewR2Store(R2Config{});if err!=nil{t.Fatal(err)}
	if s.Configured(){t.Fatal("empty R2 config must be disabled")}
}

func TestR2RejectsPartialConfig(t *testing.T){
	if _,err:=NewR2Store(R2Config{Endpoint:"https://example.invalid"});err==nil{t.Fatal("expected partial config error")}
}

func TestCanonicalURIEncodesSegments(t *testing.T){
	got:=canonicalURI("/bucket/folder/a b+c.pdf")
	want:="/bucket/folder/a%20b%2Bc.pdf"
	if got!=want{t.Fatalf("got %q want %q",got,want)}
}
