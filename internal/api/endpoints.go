package api

import (
	"net/http"
	"pulseroute/internal/security"
)

type endpointInput struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Secret      string `json:"secret"`
	Active      *bool  `json:"active"`
	MaxAttempts int    `json:"max_attempts"`
	BaseDelay   int    `json:"base_delay_seconds"`
	Rate        int    `json:"rate_per_second"`
}

func (s *Server) validateEndpoint(w http.ResponseWriter, r *http.Request, v *endpointInput) bool {
	if !validName(v.Name) {
		fail(w, r, 400, "INVALID_NAME", "name must contain 1..100 characters")
		return false
	}
	if e := security.ValidateURL(v.URL, s.Config.AllowPrivate); e != nil {
		fail(w, r, 400, "INVALID_URL", e.Error())
		return false
	}
	if v.MaxAttempts < 1 || v.MaxAttempts > 10 || v.BaseDelay < 1 || v.BaseDelay > 3600 || v.Rate < 1 || v.Rate > 1000 {
		fail(w, r, 400, "INVALID_POLICY", "attempts 1..10, base delay 1..3600, rate 1..1000 required")
		return false
	}
	if v.Secret != "" && (len(v.Secret) < 16 || len(v.Secret) > 256) {
		fail(w, r, 400, "INVALID_SECRET", "secret must contain 16..256 bytes")
		return false
	}
	return true
}
func (s *Server) endpoints(w http.ResponseWriter, r *http.Request) {
	if !s.owner(r, r.PathValue("id")) {
		fail(w, r, 404, "NOT_FOUND", "project not found")
		return
	}
	l, o := page(r)
	s.rows(w, r, "SELECT id,name,url,active,max_attempts,base_delay_seconds,rate_per_second,created_at FROM endpoints WHERE project_id=$1 AND deleted_at IS NULL ORDER BY created_at DESC,id LIMIT $2 OFFSET $3", r.PathValue("id"), l, o)
}
func (s *Server) createEndpoint(w http.ResponseWriter, r *http.Request) {
	if !s.owner(r, r.PathValue("id")) {
		fail(w, r, 404, "NOT_FOUND", "project not found")
		return
	}
	v := endpointInput{MaxAttempts: 5, BaseDelay: 5, Rate: 10}
	if !decode(w, r, &v) || !s.validateEndpoint(w, r, &v) {
		return
	}
	if v.Secret == "" {
		v.Secret = security.Token("whsec_")
	}
	cipher, e := security.Encrypt(s.Config.EncryptionKey, v.Secret)
	if e != nil {
		s.dbError(w, r, e)
		return
	}
	id := security.Token("ep_")
	active := true
	if v.Active != nil {
		active = *v.Active
	}
	_, e = s.DB.Exec(r.Context(), "INSERT INTO endpoints(id,project_id,name,url,secret_cipher,active,max_attempts,base_delay_seconds,rate_per_second) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)", id, r.PathValue("id"), v.Name, v.URL, cipher, active, v.MaxAttempts, v.BaseDelay, v.Rate)
	if e != nil {
		s.dbError(w, r, e)
		return
	}
	write(w, 201, map[string]string{"id": id, "secret": v.Secret})
}
func (s *Server) updateEndpoint(w http.ResponseWriter, r *http.Request) {
	var v endpointInput
	var cipher string
	var active bool
	e := s.DB.QueryRow(r.Context(), "SELECT e.name,e.url,e.secret_cipher,e.active,e.max_attempts,e.base_delay_seconds,e.rate_per_second FROM endpoints e JOIN projects p ON p.id=e.project_id WHERE e.id=$1 AND p.owner_id=$2 AND e.deleted_at IS NULL", r.PathValue("id"), user(r)).Scan(&v.Name, &v.URL, &cipher, &active, &v.MaxAttempts, &v.BaseDelay, &v.Rate)
	if e != nil {
		s.dbError(w, r, e)
		return
	}
	v.Active = &active
	if !decode(w, r, &v) || !s.validateEndpoint(w, r, &v) {
		return
	}
	if v.Secret != "" {
		cipher, e = security.Encrypt(s.Config.EncryptionKey, v.Secret)
		if e != nil {
			s.dbError(w, r, e)
			return
		}
	}
	if v.Active == nil {
		fail(w, r, 400, "INVALID_ACTIVE", "active must be boolean")
		return
	}
	_, e = s.DB.Exec(r.Context(), "UPDATE endpoints SET name=$2,url=$3,secret_cipher=$4,active=$5,max_attempts=$6,base_delay_seconds=$7,rate_per_second=$8 WHERE id=$1", r.PathValue("id"), v.Name, v.URL, cipher, *v.Active, v.MaxAttempts, v.BaseDelay, v.Rate)
	if e != nil {
		s.dbError(w, r, e)
		return
	}
	write(w, 200, map[string]string{"id": r.PathValue("id")})
}
func (s *Server) deleteEndpoint(w http.ResponseWriter, r *http.Request) {
	tag, e := s.DB.Exec(r.Context(), "UPDATE endpoints e SET active=false,deleted_at=now() FROM projects p WHERE e.project_id=p.id AND p.owner_id=$1 AND e.id=$2 AND e.deleted_at IS NULL", user(r), r.PathValue("id"))
	if e != nil {
		s.dbError(w, r, e)
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, r, 404, "NOT_FOUND", "endpoint not found")
		return
	}
	w.WriteHeader(204)
}
