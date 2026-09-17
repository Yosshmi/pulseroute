package api

import (
	"net/http"
	"pulseroute/internal/security"
	"strings"
)

type named struct {
	Name string `json:"name"`
}

func validName(s string) bool { return strings.TrimSpace(s) != "" && len(s) <= 100 }
func (s *Server) projects(w http.ResponseWriter, r *http.Request) {
	l, o := page(r)
	s.rows(w, r, "SELECT id,name,created_at FROM projects WHERE owner_id=$1 ORDER BY created_at DESC,id LIMIT $2 OFFSET $3", user(r), l, o)
}
func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	var n named
	if !decode(w, r, &n) {
		return
	}
	if !validName(n.Name) {
		fail(w, r, 400, "INVALID_NAME", "name must contain 1..100 characters")
		return
	}
	id := security.Token("prj_")
	if _, e := s.DB.Exec(r.Context(), "INSERT INTO projects(id,owner_id,name) VALUES($1,$2,$3)", id, user(r), strings.TrimSpace(n.Name)); e != nil {
		s.dbError(w, r, e)
		return
	}
	write(w, 201, map[string]string{"id": id, "name": n.Name})
}
func (s *Server) project(w http.ResponseWriter, r *http.Request) {
	s.one(w, r, "SELECT json_build_object('id',id,'name',name,'created_at',created_at) FROM projects WHERE id=$1 AND owner_id=$2", r.PathValue("id"), user(r))
}
func (s *Server) keys(w http.ResponseWriter, r *http.Request) {
	if !s.owner(r, r.PathValue("id")) {
		fail(w, r, 404, "NOT_FOUND", "project not found")
		return
	}
	l, o := page(r)
	s.rows(w, r, "SELECT id,name,prefix,revoked_at,created_at FROM api_keys WHERE project_id=$1 ORDER BY created_at DESC,id LIMIT $2 OFFSET $3", r.PathValue("id"), l, o)
}
func (s *Server) createKey(w http.ResponseWriter, r *http.Request) {
	if !s.owner(r, r.PathValue("id")) {
		fail(w, r, 404, "NOT_FOUND", "project not found")
		return
	}
	var n named
	if !decode(w, r, &n) {
		return
	}
	if !validName(n.Name) {
		fail(w, r, 400, "INVALID_NAME", "name must contain 1..100 characters")
		return
	}
	id, key := security.Token("key_"), security.Token("pr_live_")
	if _, e := s.DB.Exec(r.Context(), "INSERT INTO api_keys(id,project_id,name,key_hash,prefix) VALUES($1,$2,$3,$4,$5)", id, r.PathValue("id"), n.Name, security.Hash(key), key[:16]); e != nil {
		s.dbError(w, r, e)
		return
	}
	write(w, 201, map[string]string{"id": id, "key": key, "prefix": key[:16]})
}
func (s *Server) deleteKey(w http.ResponseWriter, r *http.Request) {
	tag, e := s.DB.Exec(r.Context(), "UPDATE api_keys k SET revoked_at=now() FROM projects p WHERE k.project_id=p.id AND p.owner_id=$1 AND k.id=$2", user(r), r.PathValue("id"))
	if e != nil {
		s.dbError(w, r, e)
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, r, 404, "NOT_FOUND", "API key not found")
		return
	}
	w.WriteHeader(204)
}
