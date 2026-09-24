package auth

import (
	"bytes"
	"testing"
)

func TestSessionTokenHashRoundTrip(t *testing.T){
	raw,hash,err:=NewSessionToken();if err!=nil{t.Fatal(err)}
	if raw==""||len(hash)!=32{t.Fatalf("unexpected token/hash lengths raw=%d hash=%d",len(raw),len(hash))}
	again,err:=HashSessionToken(raw);if err!=nil{t.Fatal(err)}
	if !bytes.Equal(hash,again){t.Fatal("session token hash mismatch")}
}

func TestSessionTokenRejectsInvalid(t *testing.T){
	if _,err:=HashSessionToken("not-a-token");err==nil{t.Fatal("expected invalid token error")}
}
