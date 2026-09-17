package api

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeRejectsTrailingAndUnknown(t *testing.T) {
	for _, body := range []string{`{"name":"a"} {}`, `{"unexpected":true}`, strings.Repeat("x", (1<<20)+1)} {
		r := httptest.NewRequest("POST", "/", strings.NewReader(body))
		w := httptest.NewRecorder()
		var n named
		if decode(w, r, &n) || w.Code != 400 || !strings.Contains(w.Body.String(), "INVALID_JSON") {
			t.Fatal("invalid error contract")
		}
	}
}
func TestPagination(t *testing.T) {
	r := httptest.NewRequest("GET", "/?limit=999&offset=-1", nil)
	l, o := page(r)
	if l != 25 || o != 0 {
		t.Fatal(l, o)
	}
}
