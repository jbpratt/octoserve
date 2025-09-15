package metrics

import (
	"crypto/subtle"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Handler creates an HTTP handler for the /metrics endpoint
func Handler(registry *Registry, username, password string) http.Handler {
	if registry == nil {
		// Return a handler that returns 404 if metrics are disabled
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		})
	}

	promHandler := promhttp.HandlerFor(registry.GetRegistry(), promhttp.HandlerOpts{
		ErrorLog:      nil, // Use default error handling
		ErrorHandling: promhttp.ContinueOnError,
	})

	// If no authentication is configured, return the handler directly
	if username == "" || password == "" {
		return promHandler
	}

	// Wrap with basic authentication
	return basicAuth(promHandler, username, password)
}

// basicAuth provides HTTP Basic Authentication middleware
func basicAuth(handler http.Handler, username, password string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()

		// Use constant-time comparison to prevent timing attacks
		if !ok ||
			subtle.ConstantTimeCompare([]byte(user), []byte(username)) != 1 ||
			subtle.ConstantTimeCompare([]byte(pass), []byte(password)) != 1 {

			w.Header().Set("WWW-Authenticate", `Basic realm="Metrics"`)
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Unauthorized\n"))
			return
		}

		handler.ServeHTTP(w, r)
	})
}
