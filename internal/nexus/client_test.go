package nexus

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCheckAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, _ := r.BasicAuth()
		switch {
		case r.URL.Path != "/service/rest/v1/repositories":
			w.WriteHeader(http.StatusNotFound)
		case user == "admin" && pass == "good":
			w.Write([]byte("[]"))
		case user == "limited" && pass == "good":
			w.WriteHeader(http.StatusForbidden)
		default:
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer srv.Close()

	for _, tc := range []struct {
		user, pass string
		want       error
	}{
		{"admin", "good", nil},
		{"limited", "good", nil},
		{"admin", "bad", ErrUnauthorized},
	} {
		err := New(srv.URL+"/", tc.user, tc.pass, time.Second).CheckAuth(t.Context())
		if !errors.Is(err, tc.want) {
			t.Errorf("CheckAuth(%s/%s) = %v, want %v", tc.user, tc.pass, err, tc.want)
		}
	}
}
