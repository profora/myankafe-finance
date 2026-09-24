package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/profora/myankafe-finance/backend/internal/ids"
)

type UserCredential struct {
	User         User
	PasswordHash string
}

type UserSession struct {
	ID         string
	PublicID   string
	UserID     string
	ExpiresAt  time.Time
	LastSeenAt time.Time
}

type UserSessionInfo struct {
	PublicID   string    `json:"id"`
	UserAgent  *string   `json:"user_agent"`
	IPAddress  *string   `json:"ip_address"`
	ExpiresAt  time.Time `json:"expires_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	CreatedAt  time.Time `json:"created_at"`
}

func (s *Store) CredentialByUsername(ctx context.Context, username string) (UserCredential, error) {
	var out UserCredential
	err := s.Pool.QueryRow(ctx, `
SELECT u.id::text,u.public_id::text,u.username,u.display_name,uc.password_hash
FROM users u
JOIN user_credentials uc ON uc.user_id=u.id
WHERE u.username=$1 AND u.status='ACTIVE'`, username).
		Scan(&out.User.ID,&out.User.PublicID,&out.User.Username,&out.User.DisplayName,&out.PasswordHash)
	return out, err
}

func (s *Store) CredentialByUserID(ctx context.Context, userID string) (string, error) {
	var hash string
	err := s.Pool.QueryRow(ctx, `SELECT password_hash FROM user_credentials WHERE user_id=$1`, userID).Scan(&hash)
	return hash, err
}

func (s *Store) UpsertPasswordHash(ctx context.Context, userID, passwordHash string) error {
	_, err := s.Pool.Exec(ctx, `
INSERT INTO user_credentials(user_id,password_hash)
VALUES($1,$2)
ON CONFLICT(user_id) DO UPDATE
SET password_hash=EXCLUDED.password_hash,
    password_changed_at=now(),
    updated_at=now()`, userID,passwordHash)
	return err
}

func (s *Store) CreateSession(ctx context.Context,userID string,tokenHash []byte,userAgent,ip string,expiresAt time.Time)(UserSession,error){
	id,err:=ids.UUIDv7();if err!=nil{return UserSession{},err}
	pub,err:=ids.ULID();if err!=nil{return UserSession{},err}
	var out UserSession
	err=s.Pool.QueryRow(ctx,`
INSERT INTO user_sessions(id,public_id,user_id,token_hash,user_agent,ip_address,expires_at)
VALUES($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,'')::inet,$7)
RETURNING id::text,public_id::text,user_id::text,expires_at,last_seen_at`,
		id,pub,userID,tokenHash,userAgent,ip,expiresAt).
		Scan(&out.ID,&out.PublicID,&out.UserID,&out.ExpiresAt,&out.LastSeenAt)
	return out,err
}

func (s *Store) ResolveSession(ctx context.Context,tokenHash []byte)(User,UserSession,error){
	var u User
	var sess UserSession
	err:=s.Pool.QueryRow(ctx,`
SELECT u.id::text,u.public_id::text,u.username,u.display_name,
       us.id::text,COALESCE(us.public_id::text,''),us.user_id::text,us.expires_at,us.last_seen_at
FROM user_sessions us
JOIN users u ON u.id=us.user_id
WHERE us.token_hash=$1
  AND us.revoked_at IS NULL
  AND us.expires_at>now()
  AND u.status='ACTIVE'`,tokenHash).
		Scan(&u.ID,&u.PublicID,&u.Username,&u.DisplayName,&sess.ID,&sess.PublicID,&sess.UserID,&sess.ExpiresAt,&sess.LastSeenAt)
	return u,sess,err
}

func (s *Store) TouchSession(ctx context.Context,sessionID string) error {
	_,err:=s.Pool.Exec(ctx,`UPDATE user_sessions SET last_seen_at=now() WHERE id=$1 AND revoked_at IS NULL AND expires_at>now()`,sessionID)
	return err
}

func (s *Store) RevokeSession(ctx context.Context,tokenHash []byte) error {
	_,err:=s.Pool.Exec(ctx,`UPDATE user_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE token_hash=$1`,tokenHash)
	return err
}

func (s *Store) RevokeAllSessions(ctx context.Context,userID string) error {
	_,err:=s.Pool.Exec(ctx,`UPDATE user_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE user_id=$1 AND revoked_at IS NULL`,userID)
	return err
}

func (s *Store) ChangePasswordAndReplaceSessions(
	ctx context.Context,
	user User,
	passwordHash string,
	tokenHash []byte,
	userAgent, ip string,
	expiresAt time.Time,
) (UserSession, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return UserSession{}, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
UPDATE user_credentials
SET password_hash=$2,password_changed_at=now(),updated_at=now()
WHERE user_id=$1`, user.ID, passwordHash); err != nil {
		return UserSession{}, err
	}
	if _, err := tx.Exec(ctx, `
UPDATE user_sessions
SET revoked_at=COALESCE(revoked_at,now())
WHERE user_id=$1 AND revoked_at IS NULL`, user.ID); err != nil {
		return UserSession{}, err
	}

	sessionID, err := ids.UUIDv7()
	if err != nil {
		return UserSession{}, err
	}
	sessionPublicID, err := ids.ULID()
	if err != nil {
		return UserSession{}, err
	}
	var session UserSession
	if err := tx.QueryRow(ctx, `
INSERT INTO user_sessions(id,public_id,user_id,token_hash,user_agent,ip_address,expires_at)
VALUES($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,'')::inet,$7)
RETURNING id::text,public_id::text,user_id::text,expires_at,last_seen_at`,
		sessionID,sessionPublicID,user.ID,tokenHash,userAgent,ip,expiresAt).
		Scan(&session.ID,&session.PublicID,&session.UserID,&session.ExpiresAt,&session.LastSeenAt); err != nil {
		return UserSession{}, err
	}

	auditID, err := ids.UUIDv7()
	if err != nil {
		return UserSession{}, err
	}
	auditPublicID, err := ids.ULID()
	if err != nil {
		return UserSession{}, err
	}
	meta, err := json.Marshal(map[string]any{"session_id":session.ID})
	if err != nil {
		return UserSession{}, err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO audit_events(
 id,public_id,actor_type,actor_user_id,action,resource_type,resource_id,
 outcome,source,metadata
) VALUES($1,$2,'USER',$3,'AUTH_PASSWORD_CHANGED','USER',$3,'SUCCESS','WEB',$4::jsonb)`,
		auditID,auditPublicID,user.ID,string(meta)); err != nil {
		return UserSession{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return UserSession{}, err
	}
	return session, nil
}

func (s *Store) AuditAuth(ctx context.Context,userID *string,action,outcome string,metadata map[string]any) error {
	id,err:=ids.UUIDv7();if err!=nil{return err}
	pub,err:=ids.ULID();if err!=nil{return err}
	body,err:=json.Marshal(metadata);if err!=nil{return err}
	actorType:="ANONYMOUS"
	var actor any
	if userID!=nil&&*userID!=""{actorType="USER";actor=*userID}
	_,err=s.Pool.Exec(ctx,`
INSERT INTO audit_events(
 id,public_id,actor_type,actor_user_id,action,resource_type,outcome,source,metadata
) VALUES($1,$2,$3,$4,$5,'AUTH',$6,'WEB',$7::jsonb)`,
		id,pub,actorType,actor,action,outcome,string(body))
	return err
}

func (s *Store) ResetUserPassword(ctx context.Context,actor User,targetPublicID,passwordHash string) error {
	if passwordHash==""{return fmt.Errorf("password hash is required")}
	tx,err:=s.Pool.Begin(ctx);if err!=nil{return err}
	defer tx.Rollback(ctx)

	var targetID string
	if err:=tx.QueryRow(ctx,`SELECT id::text FROM users WHERE public_id=$1 AND status='ACTIVE' FOR UPDATE`,targetPublicID).Scan(&targetID);err!=nil{return err}
	if _,err:=tx.Exec(ctx,`
INSERT INTO user_credentials(user_id,password_hash)
VALUES($1,$2)
ON CONFLICT(user_id) DO UPDATE
SET password_hash=EXCLUDED.password_hash,
    password_changed_at=now(),
    updated_at=now()`,targetID,passwordHash);err!=nil{return err}
	if _,err:=tx.Exec(ctx,`UPDATE user_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE user_id=$1 AND revoked_at IS NULL`,targetID);err!=nil{return err}

	auditID,err:=ids.UUIDv7();if err!=nil{return err}
	auditPublic,err:=ids.ULID();if err!=nil{return err}
	after,err:=json.Marshal(map[string]any{"target_user_id":targetPublicID,"sessions_revoked":true});if err!=nil{return err}
	if _,err:=tx.Exec(ctx,`
INSERT INTO audit_events(
 id,public_id,actor_type,actor_user_id,action,resource_type,resource_public_id,outcome,source,after_data
) VALUES($1,$2,'USER',$3,'USER_PASSWORD_RESET','USER',$4,'SUCCESS','WEB',$5::jsonb)`,
		auditID,auditPublic,actor.ID,targetPublicID,string(after));err!=nil{return err}
	return tx.Commit(ctx)
}

func (s *Store) CreateOrUpdateUserPassword(ctx context.Context,username,passwordHash string) error {
	var userID string
	if err:=s.Pool.QueryRow(ctx,`SELECT id::text FROM users WHERE username=$1`,username).Scan(&userID);err!=nil{
		return fmt.Errorf("load user credential target: %w",err)
	}
	return s.UpsertPasswordHash(ctx,userID,passwordHash)
}


func (s *Store) ListActiveSessions(ctx context.Context,userID string)([]UserSessionInfo,error){
	rows,err:=s.Pool.Query(ctx,`
SELECT public_id::text,user_agent,host(ip_address)::text,expires_at,last_seen_at,created_at
FROM user_sessions
WHERE user_id=$1
  AND public_id IS NOT NULL
  AND revoked_at IS NULL
  AND expires_at>now()
ORDER BY created_at DESC`,userID)
	if err!=nil{return nil,err}
	defer rows.Close()
	out:=[]UserSessionInfo{}
	for rows.Next(){
		var item UserSessionInfo
		if err:=rows.Scan(&item.PublicID,&item.UserAgent,&item.IPAddress,&item.ExpiresAt,&item.LastSeenAt,&item.CreatedAt);err!=nil{return nil,err}
		out=append(out,item)
	}
	return out,rows.Err()
}

func (s *Store) RevokeSessionByPublicID(ctx context.Context,userID,sessionPublicID string)(string,error){
	var internalID string
	err:=s.Pool.QueryRow(ctx,`
UPDATE user_sessions
SET revoked_at=COALESCE(revoked_at,now())
WHERE user_id=$1
  AND public_id=$2
  AND revoked_at IS NULL
RETURNING id::text`,userID,sessionPublicID).Scan(&internalID)
	return internalID,err
}

func (s *Store) RevokeOtherSessions(ctx context.Context,userID,currentSessionID string)(int64,error){
	tag,err:=s.Pool.Exec(ctx,`
UPDATE user_sessions
SET revoked_at=COALESCE(revoked_at,now())
WHERE user_id=$1
  AND id<>$2
  AND revoked_at IS NULL
  AND expires_at>now()`,userID,currentSessionID)
	if err!=nil{return 0,err}
	return tag.RowsAffected(),nil
}
