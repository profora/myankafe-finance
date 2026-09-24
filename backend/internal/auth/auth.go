package auth

import (
  "context"
  "net/http"
  "strings"
)

type Principal struct{ PublicID string }
type key struct{}

func Middleware(mode string) func(http.Handler) http.Handler {
  return func(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
      if mode!="dev" { http.Error(w,"production authentication not configured",http.StatusServiceUnavailable); return }
      h:=r.Header.Get("Authorization")
      if !strings.HasPrefix(h,"Bearer dev:") { http.Error(w,"unauthorized",http.StatusUnauthorized); return }
      id:=strings.TrimPrefix(h,"Bearer dev:")
      if len(id)!=26 { http.Error(w,"invalid development principal",http.StatusUnauthorized); return }
      next.ServeHTTP(w,r.WithContext(context.WithValue(r.Context(),key{},Principal{PublicID:id})))
    })
  }
}

func From(ctx context.Context)(Principal,bool){ p,ok:=ctx.Value(key{}).(Principal); return p,ok }
