package middleware

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/jbpratt/octoserve/internal/api"
	"github.com/jbpratt/octoserve/internal/errors"
)

var (
	// Repository name pattern from OCI spec
	repositoryNameRegex = regexp.MustCompile(`^[a-z0-9]+((\.|_|__|-+)[a-z0-9]+)*(\/[a-z0-9]+((\.|_|__|-+)[a-z0-9]+)*)*$`)

	// Tag pattern from OCI spec
	tagRegex = regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9._-]{0,127}$`)

	// Digest pattern
	digestRegex = regexp.MustCompile(`^[a-zA-Z0-9+.-]+:[a-fA-F0-9]+$`)
)

// ValidateRepository validates repository names
func ValidateRepository(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := api.GetParam(r, "name")
		if name == "" {
			next.ServeHTTP(w, r)
			return
		}

		if !isValidRepositoryName(name) {
			errors.WriteErrorResponse(w, http.StatusBadRequest,
				errors.NameInvalid(name))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ValidateReference validates manifest references (tags or digests)
func ValidateReference(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reference := api.GetParam(r, "reference")
		if reference == "" {
			next.ServeHTTP(w, r)
			return
		}

		if !isValidReference(reference) {
			errors.WriteErrorResponse(w, http.StatusBadRequest,
				errors.NewOCIError("REFERENCE_INVALID", "invalid reference", reference))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ValidateDigest validates blob digests
func ValidateDigest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		digest := api.GetParam(r, "digest")
		if digest == "" {
			next.ServeHTTP(w, r)
			return
		}

		if !isValidDigest(digest) {
			errors.WriteErrorResponse(w, http.StatusBadRequest,
				errors.DigestInvalid(digest))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// SetCORSHeaders adds CORS headers for browser compatibility
func SetCORSHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Content-Length, Content-Range, Docker-Content-Digest")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// SetDockerHeaders adds Docker-compatible headers
func SetDockerHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
		next.ServeHTTP(w, r)
	})
}

// isValidRepositoryName validates repository names according to OCI spec
func isValidRepositoryName(name string) bool {
	if len(name) == 0 || len(name) > 255 {
		return false
	}

	// Check for invalid characters or patterns
	if strings.Contains(name, "..") || strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") {
		return false
	}

	return repositoryNameRegex.MatchString(name)
}

// isValidReference validates manifest references (tags or digests)
func isValidReference(reference string) bool {
	if len(reference) == 0 {
		return false
	}

	// Check if it's a digest
	if strings.Contains(reference, ":") {
		return isValidDigest(reference)
	}

	// Check if it's a tag
	return isValidTag(reference)
}

// isValidTag validates tag names according to OCI spec
func isValidTag(tag string) bool {
	if len(tag) == 0 || len(tag) > 128 {
		return false
	}

	return tagRegex.MatchString(tag)
}

// isValidDigest validates digest format
func isValidDigest(digest string) bool {
	if len(digest) == 0 {
		return false
	}

	return digestRegex.MatchString(digest)
}
