package objectstore

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

type Store interface {
	Configured() bool
	Put(ctx context.Context,key,contentType string,body []byte) error
	Get(ctx context.Context,key string)([]byte,string,error)
	Delete(ctx context.Context,key string) error
}

type R2Config struct {
	Endpoint  string
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string
}

type R2Store struct {
	cfg R2Config
	base *url.URL
	client *http.Client
}

func NewR2Store(cfg R2Config)(*R2Store,error){
	cfg.Endpoint=strings.TrimRight(strings.TrimSpace(cfg.Endpoint),"/")
	cfg.Bucket=strings.Trim(strings.TrimSpace(cfg.Bucket),"/")
	cfg.Region=strings.TrimSpace(cfg.Region)
	if cfg.Region==""{cfg.Region="auto"}
	if cfg.Endpoint==""&&cfg.Bucket==""&&cfg.AccessKey==""&&cfg.SecretKey==""{
		return &R2Store{cfg:cfg,client:&http.Client{Timeout:45*time.Second}},nil
	}
	if cfg.Endpoint==""||cfg.Bucket==""||cfg.AccessKey==""||cfg.SecretKey==""{
		return nil,errors.New("R2 endpoint, bucket, access key and secret key must be configured together")
	}
	base,err:=url.Parse(cfg.Endpoint)
	if err!=nil||base.Scheme==""||base.Host==""{
		return nil,errors.New("R2_ENDPOINT must be an absolute http(s) URL")
	}
	return &R2Store{cfg:cfg,base:base,client:&http.Client{Timeout:45*time.Second}},nil
}

func (s *R2Store) Configured() bool {
	return s!=nil&&s.base!=nil&&s.cfg.Bucket!=""&&s.cfg.AccessKey!=""&&s.cfg.SecretKey!=""
}

func (s *R2Store) Put(ctx context.Context,key,contentType string,body []byte) error {
	if !s.Configured(){return errors.New("R2 attachment storage is not configured")}
	if strings.TrimSpace(contentType)==""{contentType="application/octet-stream"}
	req,err:=s.newSignedRequest(ctx,http.MethodPut,key,contentType,body);if err!=nil{return err}
	resp,err:=s.client.Do(req);if err!=nil{return err};defer resp.Body.Close()
	if resp.StatusCode<200||resp.StatusCode>=300{
		b,_:=io.ReadAll(io.LimitReader(resp.Body,4096))
		return fmt.Errorf("R2 PUT failed: %s: %s",resp.Status,strings.TrimSpace(string(b)))
	}
	return nil
}

func (s *R2Store) Get(ctx context.Context,key string)([]byte,string,error){
	if !s.Configured(){return nil,"",errors.New("R2 attachment storage is not configured")}
	req,err:=s.newSignedRequest(ctx,http.MethodGet,key,"",nil);if err!=nil{return nil,"",err}
	resp,err:=s.client.Do(req);if err!=nil{return nil,"",err};defer resp.Body.Close()
	if resp.StatusCode<200||resp.StatusCode>=300{
		b,_:=io.ReadAll(io.LimitReader(resp.Body,4096))
		return nil,"",fmt.Errorf("R2 GET failed: %s: %s",resp.Status,strings.TrimSpace(string(b)))
	}
	body,err:=io.ReadAll(resp.Body);if err!=nil{return nil,"",err}
	ct:=strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"),";")[0])
	if ct==""{ct=http.DetectContentType(body)}
	return body,ct,nil
}

func (s *R2Store) Delete(ctx context.Context,key string) error {
	if !s.Configured(){return errors.New("R2 attachment storage is not configured")}
	req,err:=s.newSignedRequest(ctx,http.MethodDelete,key,"",nil);if err!=nil{return err}
	resp,err:=s.client.Do(req);if err!=nil{return err};defer resp.Body.Close()
	if resp.StatusCode==http.StatusNotFound{return nil}
	if resp.StatusCode<200||resp.StatusCode>=300{
		b,_:=io.ReadAll(io.LimitReader(resp.Body,4096))
		return fmt.Errorf("R2 DELETE failed: %s: %s",resp.Status,strings.TrimSpace(string(b)))
	}
	return nil
}

func (s *R2Store) newSignedRequest(ctx context.Context,method,key,contentType string,body []byte)(*http.Request,error){
	u:=*s.base
	u.Path=path.Join(u.Path,s.cfg.Bucket,strings.TrimLeft(key,"/"))
	payloadHash:=sha256Hex(body)
	var reader io.Reader
	if body!=nil{reader=bytes.NewReader(body)}
	req,err:=http.NewRequestWithContext(ctx,method,u.String(),reader);if err!=nil{return nil,err}
	if contentType!=""{req.Header.Set("Content-Type",contentType)}
	now:=time.Now().UTC()
	amzDate:=now.Format("20060102T150405Z")
	date:=now.Format("20060102")
	req.Header.Set("X-Amz-Date",amzDate)
	req.Header.Set("X-Amz-Content-Sha256",payloadHash)
	signedHeaders:="host;x-amz-content-sha256;x-amz-date"
	canonicalHeaders:="host:"+req.URL.Host+"
"+"x-amz-content-sha256:"+payloadHash+"
"+"x-amz-date:"+amzDate+"
"
	canonicalRequest:=strings.Join([]string{
		method,canonicalURI(req.URL.Path),"",canonicalHeaders,signedHeaders,payloadHash,
	},"
")
	scope:=date+"/"+s.cfg.Region+"/s3/aws4_request"
	stringToSign:="AWS4-HMAC-SHA256
"+amzDate+"
"+scope+"
"+sha256Hex([]byte(canonicalRequest))
	signingKey:=deriveSigningKey(s.cfg.SecretKey,date,s.cfg.Region,"s3")
	signature:=hex.EncodeToString(hmacSHA256(signingKey,[]byte(stringToSign)))
	req.Header.Set("Authorization","AWS4-HMAC-SHA256 Credential="+s.cfg.AccessKey+"/"+scope+", SignedHeaders="+signedHeaders+", Signature="+signature)
	return req,nil
}

func canonicalURI(p string) string {
	if p==""{return "/"}
	parts:=strings.Split(p,"/")
	for i,part:=range parts{parts[i]=awsPercentEncode(part)}
	out:=strings.Join(parts,"/")
	if !strings.HasPrefix(out,"/"){out="/"+out}
	return out
}

func awsPercentEncode(v string) string {
	const hexChars="0123456789ABCDEF"
	var b strings.Builder
	for i:=0;i<len(v);i++{
		c:=v[i]
		if (c>='A'&&c<='Z')||(c>='a'&&c<='z')||(c>='0'&&c<='9')||c=='-'||c=='_'||c=='.'||c=='~'{
			b.WriteByte(c);continue
		}
		b.WriteByte('%');b.WriteByte(hexChars[c>>4]);b.WriteByte(hexChars[c&0x0f])
	}
	return b.String()
}

func sha256Hex(b []byte) string { h:=sha256.Sum256(b);return hex.EncodeToString(h[:]) }
func hmacSHA256(key,data []byte)[]byte{h:=hmac.New(sha256.New,key);_,_=h.Write(data);return h.Sum(nil)}
func deriveSigningKey(secret,date,region,service string)[]byte{
	kDate:=hmacSHA256([]byte("AWS4"+secret),[]byte(date))
	kRegion:=hmacSHA256(kDate,[]byte(region))
	kService:=hmacSHA256(kRegion,[]byte(service))
	return hmacSHA256(kService,[]byte("aws4_request"))
}
