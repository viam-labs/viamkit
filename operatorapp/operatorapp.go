// Package operatorapp serves a Viam module's operator web app: it hosts a
// static frontend and injects the machine credentials the browser Viam SDK
// reads from cookies to authenticate to the cell — so a plain HTML/JS page
// can drive the module without a separate auth proxy.
//
// It serves the frontend you give it; it does not build the frontend. Point
// it at a filesystem (embedded or on disk) holding your index.html and
// assets:
//
//	//go:embed all:static
//	var assets embed.FS
//
//	func main() {
//		app, err := fs.Sub(assets, "static")
//		if err != nil {
//			log.Fatal(err)
//		}
//		log.Println("operator app on http://localhost:8080")
//		log.Fatal(operatorapp.ListenAndServe(":8080", app))
//	}
//
// Credentials default to the standard VIAM_ROBOT_PART_ID, VIAM_ROBOT_FQDN,
// VIAM_API_KEY_ID, and VIAM_API_KEY environment variables — the same names
// the Viam app and "viam module local-app-testing" set. Override them with
// WithCredentials (for tests or a non-environment source).
//
// The credential cookies are intentionally readable by JavaScript (they are
// not HttpOnly): the browser SDK reads them from document.cookie. The cookies
// carry an API key in clear text, so serve over a trusted/local connection.
package operatorapp

import (
	"io/fs"
	"net/http"
	"os"
	"time"
)

// Credentials are the machine-connection values the browser Viam SDK reads
// from cookies to authenticate to the cell.
type Credentials struct {
	PartID   string // machine part ID  -> "part-id" cookie
	FQDN     string // machine FQDN/host -> "host" cookie
	APIKeyID string // API key ID        -> "api-key-id" cookie
	APIKey   string // API key           -> "api-key" cookie
}

// CredentialsFromEnv reads Credentials from the standard VIAM_ROBOT_PART_ID,
// VIAM_ROBOT_FQDN, VIAM_API_KEY_ID, and VIAM_API_KEY environment variables.
func CredentialsFromEnv() Credentials {
	return Credentials{
		PartID:   os.Getenv("VIAM_ROBOT_PART_ID"),
		FQDN:     os.Getenv("VIAM_ROBOT_FQDN"),
		APIKeyID: os.Getenv("VIAM_API_KEY_ID"),
		APIKey:   os.Getenv("VIAM_API_KEY"),
	}
}

// cookies returns the credential cookies under the names the browser SDK
// expects. They are non-HttpOnly so the frontend can read them.
func (c Credentials) cookies() []*http.Cookie {
	pairs := []struct{ name, value string }{
		{"part-id", c.PartID},
		{"host", c.FQDN},
		{"api-key-id", c.APIKeyID},
		{"api-key", c.APIKey},
	}
	cookies := make([]*http.Cookie, len(pairs))
	for i, p := range pairs {
		cookies[i] = &http.Cookie{Name: p.name, Value: p.value, Path: "/"}
	}
	return cookies
}

// config is the resolved Handler configuration.
type config struct {
	creds Credentials
}

// Option configures a Handler.
type Option func(*config)

// WithCredentials sets the machine credentials served as cookies, overriding
// the environment-variable default.
func WithCredentials(c Credentials) Option {
	return func(cfg *config) { cfg.creds = c }
}

// Handler serves fsys as a static site and sets the machine-credential cookies
// on every response. Credentials default to CredentialsFromEnv; override with
// WithCredentials. The returned handler is safe for concurrent use.
func Handler(fsys fs.FS, opts ...Option) http.Handler {
	cfg := config{creds: CredentialsFromEnv()}
	for _, o := range opts {
		o(&cfg)
	}
	cookies := cfg.creds.cookies()
	files := http.FileServerFS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, ck := range cookies {
			http.SetCookie(w, ck)
		}
		files.ServeHTTP(w, r)
	})
}

// ListenAndServe serves fsys (via Handler) on addr and blocks until the server
// stops, returning the resulting error. It does not log — wrap or precede the
// call with your own logging.
func ListenAndServe(addr string, fsys fs.FS, opts ...Option) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           Handler(fsys, opts...),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return srv.ListenAndServe()
}
