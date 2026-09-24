package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/go-chi/chi/v5"
	"github.com/profora/myankafe-finance/backend/internal/ids"
	"github.com/profora/myankafe-finance/backend/internal/repository/postgres"
)

const maxAttachmentsPerUpload = 10

func (s *Server) listTransactionAttachments(w http.ResponseWriter,r *http.Request){
	a:=getAccess(r)
	items,err:=s.Store.ListTransactionAttachments(r.Context(),a.Entity.ID,chi.URLParam(r,"tx"))
	if err!=nil{fail(w,500,err);return}
	write(w,200,map[string]any{"items":items})
}

func (s *Server) uploadTransactionAttachments(w http.ResponseWriter,r *http.Request){
	a:=getAccess(r)
	if !requireRole(w,canOperateLedger(a.Role),"ledger operation access required"){return}
	if s.AttachmentStore==nil||!s.AttachmentStore.Configured(){
		fail(w,http.StatusServiceUnavailable,errors.New("R2 attachment storage is not configured"))
		return
	}

	maxRequest:=s.Config.AttachmentMaxBytes*maxAttachmentsPerUpload+(1<<20)
	r.Body=http.MaxBytesReader(w,r.Body,maxRequest)
	if err:=r.ParseMultipartForm(8<<20);err!=nil{
		fail(w,http.StatusBadRequest,fmt.Errorf("invalid multipart upload: %w",err))
		return
	}
	files:=r.MultipartForm.File["files"]
	if len(files)==0{fail(w,400,errors.New("at least one file is required"));return}
	if len(files)>maxAttachmentsPerUpload{fail(w,400,fmt.Errorf("at most %d files may be uploaded at once",maxAttachmentsPerUpload));return}

	type staged struct{
		meta postgres.NewAttachment
		body []byte
	}
	stagedFiles:=make([]staged,0,len(files))
	for _,fh:=range files{
		file,err:=fh.Open();if err!=nil{fail(w,400,err);return}
		body,readErr:=io.ReadAll(io.LimitReader(file,s.Config.AttachmentMaxBytes+1))
		_ = file.Close()
		if readErr!=nil{fail(w,400,readErr);return}
		if int64(len(body))>s.Config.AttachmentMaxBytes{
			fail(w,http.StatusRequestEntityTooLarge,fmt.Errorf("%s exceeds the %d MB attachment limit",fh.Filename,s.Config.AttachmentMaxBytes>>20))
			return
		}
		if len(body)==0{fail(w,400,fmt.Errorf("%s is empty",fh.Filename));return}

		mimeType:=strings.ToLower(strings.TrimSpace(strings.Split(fh.Header.Get("Content-Type"),";")[0]))
		detected:=strings.ToLower(strings.TrimSpace(strings.Split(http.DetectContentType(body[:minInt(len(body),512)]),";")[0]))
		if mimeType==""||mimeType=="application/octet-stream"{mimeType=detected}
		if !allowedAttachmentType(mimeType){
			fail(w,http.StatusUnsupportedMediaType,fmt.Errorf("%s has unsupported content type %s",fh.Filename,mimeType))
			return
		}
		if err:=validatePreviewAttachmentContent(mimeType,detected);err!=nil{
			fail(w,http.StatusUnsupportedMediaType,fmt.Errorf("%s: %w",fh.Filename,err))
			return
		}
		publicID,err:=ids.ULID();if err!=nil{fail(w,500,err);return}
		sum:=sha256.Sum256(body)
		filename:=safeAttachmentFilename(fh.Filename)
		key:=fmt.Sprintf("transactions/%s/%s/%s/%s",a.Entity.PublicID,chi.URLParam(r,"tx"),publicID,filename)
		stagedFiles=append(stagedFiles,staged{
			meta:postgres.NewAttachment{
				PublicID:publicID,StorageKey:key,OriginalFilename:filename,MimeType:mimeType,
				SizeBytes:int64(len(body)),SHA256Hex:hex.EncodeToString(sum[:]),
			},
			body:body,
		})
	}

	uploaded:=make([]string,0,len(stagedFiles))
	cleanup:=func(){for _,key:=range uploaded{_ = s.AttachmentStore.Delete(r.Context(),key)}}
	for _,item:=range stagedFiles{
		if err:=s.AttachmentStore.Put(r.Context(),item.meta.StorageKey,item.meta.MimeType,item.body);err!=nil{
			cleanup();fail(w,http.StatusBadGateway,err);return
		}
		uploaded=append(uploaded,item.meta.StorageKey)
	}
	meta:=make([]postgres.NewAttachment,0,len(stagedFiles))
	for _,item:=range stagedFiles{meta=append(meta,item.meta)}
	items,err:=s.Store.RegisterTransactionAttachments(r.Context(),a.User,a.Entity,chi.URLParam(r,"tx"),meta)
	if err!=nil{cleanup();fail(w,400,err);return}
	write(w,http.StatusCreated,map[string]any{"items":items})
}

func (s *Server) transactionAttachmentContent(w http.ResponseWriter,r *http.Request){
	a:=getAccess(r)
	if s.AttachmentStore==nil||!s.AttachmentStore.Configured(){
		fail(w,http.StatusServiceUnavailable,errors.New("R2 attachment storage is not configured"));return
	}
	obj,err:=s.Store.TransactionAttachmentObject(r.Context(),a.Entity.ID,chi.URLParam(r,"tx"),chi.URLParam(r,"attachment"))
	if err!=nil{fail(w,http.StatusNotFound,err);return}
	body,storedType,err:=s.AttachmentStore.Get(r.Context(),obj.StorageKey)
	if err!=nil{fail(w,http.StatusBadGateway,err);return}
	contentType:=obj.MimeType
	if strings.TrimSpace(contentType)==""{contentType=storedType}
	w.Header().Set("Content-Type",contentType)
	w.Header().Set("Content-Length",fmt.Sprintf("%d",len(body)))
	w.Header().Set("Cache-Control","private, max-age=300")
	w.Header().Set("X-Content-Type-Options","nosniff")
	disposition:="attachment"
	if strings.HasPrefix(contentType,"image/")||contentType=="application/pdf"{disposition="inline"}
	w.Header().Set("Content-Disposition",mime.FormatMediaType(disposition,map[string]string{"filename":obj.OriginalFilename}))
	w.WriteHeader(http.StatusOK)
	_,_ = w.Write(body)
}

func validatePreviewAttachmentContent(declared,detected string) error {
	switch declared {
	case "image/jpeg","image/png","image/webp","image/gif","application/pdf":
		if detected!=declared{
			return fmt.Errorf("declared %s but detected %s",declared,detected)
		}
	case "text/csv":
		if detected!="text/plain"&&detected!="text/csv"{
			return fmt.Errorf("declared text/csv but detected %s",detected)
		}
	case "text/plain":
		if detected!="text/plain"{
			return fmt.Errorf("declared text/plain but detected %s",detected)
		}
	}
	return nil
}

func allowedAttachmentType(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)){
	case "image/jpeg","image/png","image/webp","image/gif","application/pdf",
		"text/plain","text/csv",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return true
	default:
		return false
	}
}

func safeAttachmentFilename(v string) string {
	v=filepath.Base(strings.TrimSpace(v))
	if v==""||v=="."{return "attachment"}
	var b strings.Builder
	for _,r:=range v{
		if unicode.IsLetter(r)||unicode.IsDigit(r)||strings.ContainsRune("._- ()",r){b.WriteRune(r)}else{b.WriteRune('_')}
	}
	out:=strings.TrimSpace(b.String())
	runes:=[]rune(out)
	if len(runes)>180{out=string(runes[:180])}
	if out==""{return "attachment"}
	return out
}

func minInt(a,b int)int{if a<b{return a};return b}


func (s *Server) deleteTransactionAttachment(w http.ResponseWriter,r *http.Request){
	a:=getAccess(r)
	if !requireRole(w,canOperateLedger(a.Role),"ledger operation access required"){return}
	obj,err:=s.Store.SoftDeleteTransactionAttachment(
		r.Context(),a.User,a.Entity,chi.URLParam(r,"tx"),chi.URLParam(r,"attachment"),
	)
	if err!=nil{fail(w,http.StatusNotFound,err);return}

	if s.AttachmentStore!=nil&&s.AttachmentStore.Configured(){
		if err:=s.AttachmentStore.Delete(r.Context(),obj.StorageKey);err!=nil{
			_ = s.Store.Audit(r.Context(),a.User,&a.Entity,"ATTACHMENT_OBJECT_DELETE_FAILED","TRANSACTION",nil,"FAILED",map[string]any{
				"transaction_id":chi.URLParam(r,"tx"),
				"attachment_id":obj.PublicID,
				"storage_key":obj.StorageKey,
				"error":err.Error(),
			})
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) reorderTransactionAttachments(w http.ResponseWriter,r *http.Request){
	a:=getAccess(r)
	if !requireRole(w,canOperateLedger(a.Role),"ledger operation access required"){return}
	var in struct{
		AttachmentIDs []string `json:"attachment_ids"`
	}
	if err:=json.NewDecoder(r.Body).Decode(&in);err!=nil{fail(w,400,err);return}
	if len(in.AttachmentIDs)==0{fail(w,400,errors.New("attachment_ids is required"));return}
	if err:=s.Store.ReorderTransactionAttachments(r.Context(),a.User,a.Entity,chi.URLParam(r,"tx"),in.AttachmentIDs);err!=nil{
		fail(w,400,err);return
	}
	items,err:=s.Store.ListTransactionAttachments(r.Context(),a.Entity.ID,chi.URLParam(r,"tx"))
	if err!=nil{fail(w,500,err);return}
	write(w,200,map[string]any{"items":items})
}
