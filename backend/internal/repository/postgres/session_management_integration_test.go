package postgres

import (
	"testing"
	"time"
)

func TestSessionManagementUsesPublicULIDsAndRevokesOthers(t *testing.T){
	ctx,s:=integrationStore(t)
	user,_,_,_:=seedServiceEntity(t,ctx,s,"SESSIONS")

	hash1:=make([]byte,32);hash1[0]=1
	hash2:=make([]byte,32);hash2[0]=2
	now:=time.Now().UTC()
	first,err:=s.CreateSession(ctx,user.ID,hash1,"Browser One","127.0.0.1",now.Add(time.Hour))
	if err!=nil{t.Fatal(err)}
	second,err:=s.CreateSession(ctx,user.ID,hash2,"Browser Two","127.0.0.2",now.Add(time.Hour))
	if err!=nil{t.Fatal(err)}
	if len(first.PublicID)!=26||len(second.PublicID)!=26{t.Fatalf("session public ids=%q %q",first.PublicID,second.PublicID)}
	if first.PublicID==second.PublicID{t.Fatal("session public ids must be unique")}

	items,err:=s.ListActiveSessions(ctx,user.ID);if err!=nil{t.Fatal(err)}
	if len(items)!=2{t.Fatalf("active sessions=%d want 2",len(items))}

	count,err:=s.RevokeOtherSessions(ctx,user.ID,first.ID);if err!=nil{t.Fatal(err)}
	if count!=1{t.Fatalf("revoked=%d want 1",count)}
	items,err=s.ListActiveSessions(ctx,user.ID);if err!=nil{t.Fatal(err)}
	if len(items)!=1||items[0].PublicID!=first.PublicID{t.Fatalf("remaining sessions=%+v",items)}

	internal,err:=s.RevokeSessionByPublicID(ctx,user.ID,first.PublicID);if err!=nil{t.Fatal(err)}
	if internal!=first.ID{t.Fatalf("internal id mismatch %q != %q",internal,first.ID)}
	items,err=s.ListActiveSessions(ctx,user.ID);if err!=nil{t.Fatal(err)}
	if len(items)!=0{t.Fatalf("active sessions=%d want 0",len(items))}
}
