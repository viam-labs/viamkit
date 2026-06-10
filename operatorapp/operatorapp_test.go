package operatorapp

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestHandlerServesFiles(t *testing.T) {
	fsys := fstest.MapFS{"index.html": {Data: []byte("<h1>operator</h1>")}}
	h := Handler(fsys, WithCredentials(Credentials{}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if got := string(body); got != "<h1>operator</h1>" {
		t.Fatalf("body = %q, want index.html contents", got)
	}
}

func TestHandlerSetsCredentialCookies(t *testing.T) {
	creds := Credentials{PartID: "p1", FQDN: "h1", APIKeyID: "k1", APIKey: "secret"}
	h := Handler(fstest.MapFS{"index.html": {Data: []byte("ok")}}, WithCredentials(creds))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	want := map[string]string{"part-id": "p1", "host": "h1", "api-key-id": "k1", "api-key": "secret"}
	got := map[string]string{}
	for _, ck := range rec.Result().Cookies() {
		got[ck.Name] = ck.Value
		if ck.HttpOnly {
			t.Errorf("cookie %q is HttpOnly; the browser SDK must read it from JavaScript", ck.Name)
		}
	}
	for name, val := range want {
		if got[name] != val {
			t.Errorf("cookie %q = %q, want %q", name, got[name], val)
		}
	}
}

func TestCredentialsFromEnv(t *testing.T) {
	t.Setenv("VIAM_ROBOT_PART_ID", "part")
	t.Setenv("VIAM_ROBOT_FQDN", "host.tld")
	t.Setenv("VIAM_API_KEY_ID", "kid")
	t.Setenv("VIAM_API_KEY", "key")

	got := CredentialsFromEnv()
	want := Credentials{PartID: "part", FQDN: "host.tld", APIKeyID: "kid", APIKey: "key"}
	if got != want {
		t.Fatalf("CredentialsFromEnv() = %+v, want %+v", got, want)
	}
}

func TestHandlerDefaultsToEnvCredentials(t *testing.T) {
	t.Setenv("VIAM_ROBOT_PART_ID", "envpart")
	t.Setenv("VIAM_ROBOT_FQDN", "")
	t.Setenv("VIAM_API_KEY_ID", "")
	t.Setenv("VIAM_API_KEY", "")

	h := Handler(fstest.MapFS{"index.html": {Data: []byte("ok")}})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	var partID string
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == "part-id" {
			partID = ck.Value
		}
	}
	if partID != "envpart" {
		t.Fatalf("part-id cookie = %q, want envpart (from env)", partID)
	}
}
