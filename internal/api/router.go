package api

import (
	"net/http"
	"regexp"
	"strings"
)

// Route represents a single route
type Route struct {
	Method  string
	Pattern *regexp.Regexp
	Handler http.HandlerFunc
	Params  []string
}

// Router handles HTTP routing using standard library
type Router struct {
	routes []Route
}

// NewRouter creates a new router
func NewRouter() *Router {
	return &Router{
		routes: make([]Route, 0),
	}
}

// AddRoute adds a route to the router
func (r *Router) AddRoute(method, pattern string, handler http.HandlerFunc) {
	// Convert pattern to regex and extract parameter names
	regex, params := patternToRegex(pattern)
	route := Route{
		Method:  method,
		Pattern: regexp.MustCompile("^" + regex + "$"),
		Handler: handler,
		Params:  params,
	}
	r.routes = append(r.routes, route)
}

// ServeHTTP implements http.Handler
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	for _, route := range r.routes {
		if route.Method != req.Method {
			continue
		}

		matches := route.Pattern.FindStringSubmatch(req.URL.Path)
		if matches == nil {
			continue
		}

		// Extract parameters and add to request context
		if len(matches) > 1 {
			req = addParamsToRequest(req, route.Params, matches[1:])
		}

		route.Handler(w, req)
		return
	}

	// No route found
	http.NotFound(w, req)
}

// patternToRegex converts a pattern like "/v2/{name}/blobs/{digest}" to regex
func patternToRegex(pattern string) (string, []string) {
	var params []string

	// Replace {param} with named capture groups
	paramRegex := regexp.MustCompile(`\{([^}]+)\}`)

	regex := paramRegex.ReplaceAllStringFunc(pattern, func(match string) string {
		paramName := match[1 : len(match)-1] // Remove { and }
		params = append(params, paramName)

		switch paramName {
		case "name":
			// Repository name pattern: [a-z0-9]+((\.|_|__|-+)[a-z0-9]+)*(\/[a-z0-9]+((\.|_|__|-+)[a-z0-9]+)*)*
			return `([a-z0-9]+(?:(?:\.|_|__|-+)[a-z0-9]+)*(?:/[a-z0-9]+(?:(?:\.|_|__|-+)[a-z0-9]+)*)*)`
		case "reference":
			// Reference can be tag or digest
			return `([a-zA-Z0-9._-]{1,128}|sha256:[a-fA-F0-9]{64})`
		case "digest":
			// Digest pattern: algorithm:hex
			return `([a-zA-Z0-9+.-]+:[a-fA-F0-9]+)`
		case "uuid":
			// UUID pattern for upload sessions
			return `([a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12})`
		default:
			// Generic parameter
			return `([^/]+)`
		}
	})

	// Escape other regex characters
	regex = regexp.QuoteMeta(regex)

	// Restore the capture groups
	regex = strings.ReplaceAll(regex, `\(`, `(`)
	regex = strings.ReplaceAll(regex, `\)`, `)`)
	regex = strings.ReplaceAll(regex, `\[`, `[`)
	regex = strings.ReplaceAll(regex, `\]`, `]`)
	regex = strings.ReplaceAll(regex, `\+`, `+`)
	regex = strings.ReplaceAll(regex, `\*`, `*`)
	regex = strings.ReplaceAll(regex, `\?`, `?`)
	regex = strings.ReplaceAll(regex, `\|`, `|`)
	regex = strings.ReplaceAll(regex, `\{`, `{`)
	regex = strings.ReplaceAll(regex, `\}`, `}`)
	regex = strings.ReplaceAll(regex, `\:`, `:`)
	regex = strings.ReplaceAll(regex, `\-`, `-`)
	regex = strings.ReplaceAll(regex, `\.`, `.`)

	return regex, params
}

// addParamsToRequest adds route parameters to the request context
func addParamsToRequest(req *http.Request, paramNames []string, paramValues []string) *http.Request {
	if len(paramNames) != len(paramValues) {
		return req
	}

	// Store parameters in request URL values
	query := req.URL.Query()
	for i, name := range paramNames {
		query.Set("route_"+name, paramValues[i])
	}

	// Create new request with updated query parameters
	newReq := req.Clone(req.Context())
	newReq.URL.RawQuery = query.Encode()

	return newReq
}

// GetParam extracts a route parameter from the request
func GetParam(req *http.Request, name string) string {
	return req.URL.Query().Get("route_" + name)
}

// Convenience methods for adding routes
func (r *Router) GET(pattern string, handler http.HandlerFunc) {
	r.AddRoute("GET", pattern, handler)
}

func (r *Router) POST(pattern string, handler http.HandlerFunc) {
	r.AddRoute("POST", pattern, handler)
}

func (r *Router) PUT(pattern string, handler http.HandlerFunc) {
	r.AddRoute("PUT", pattern, handler)
}

func (r *Router) PATCH(pattern string, handler http.HandlerFunc) {
	r.AddRoute("PATCH", pattern, handler)
}

func (r *Router) DELETE(pattern string, handler http.HandlerFunc) {
	r.AddRoute("DELETE", pattern, handler)
}

func (r *Router) HEAD(pattern string, handler http.HandlerFunc) {
	r.AddRoute("HEAD", pattern, handler)
}
