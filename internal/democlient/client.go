package democlient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"time"
)

type Client struct {
	Base string
	HTTP *http.Client
}

func New(base string) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{Base: base, HTTP: &http.Client{Jar: jar, Timeout: 20 * time.Second}}
}
func (c *Client) Call(method, path, key string, body any) (map[string]any, error) {
	b, e := json.Marshal(body)
	if e != nil {
		return nil, e
	}
	r, e := http.NewRequest(method, c.Base+path, bytes.NewReader(b))
	if e != nil {
		return nil, e
	}
	r.Header.Set("Content-Type", "application/json")
	if key != "" {
		r.Header.Set("Authorization", "Bearer "+key)
	}
	resp, e := c.HTTP.Do(r)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	raw, e := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if e != nil {
		return nil, e
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, raw)
	}
	var out map[string]any
	if len(raw) > 0 {
		e = json.Unmarshal(raw, &out)
	}
	return out, e
}
func (c *Client) Bootstrap(email, password, endpoint, secret string) (string, string, error) {
	_, e := c.Call("POST", "/api/auth/register", "", map[string]string{"email": email, "password": password})
	if e != nil {
		if _, e = c.Call("POST", "/api/auth/login", "", map[string]string{"email": email, "password": password}); e != nil {
			return "", "", e
		}
	}
	p, e := c.Call("POST", "/api/projects", "", map[string]string{"name": "Synthetic operations"})
	if e != nil {
		return "", "", e
	}
	id := p["id"].(string)
	for _, item := range []struct{ name, suffix string }{{"Successful destination", ""}, {"Recovers after two failures", "?failures=2"}, {"Permanent failure", "?status=400"}} {
		if _, e = c.Call("POST", "/api/projects/"+id+"/endpoints", "", map[string]any{"name": item.name, "url": endpoint + item.suffix, "secret": secret, "base_delay_seconds": 1}); e != nil {
			return "", "", e
		}
	}
	k, e := c.Call("POST", "/api/projects/"+id+"/api-keys", "", map[string]string{"name": "Synthetic seed"})
	if e != nil {
		return "", "", e
	}
	return id, k["key"].(string), nil
}
