package errors

import (
	"encoding/json"
	"net/http"
)

// OCIError represents an OCI-compliant error
type OCIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

// OCIErrorResponse represents the complete error response
type OCIErrorResponse struct {
	Errors []OCIError `json:"errors"`
}

// Error codes as defined in the OCI Distribution Specification
const (
	CodeBlobUnknown         = "BLOB_UNKNOWN"
	CodeBlobUploadInvalid   = "BLOB_UPLOAD_INVALID"
	CodeBlobUploadUnknown   = "BLOB_UPLOAD_UNKNOWN"
	CodeDigestInvalid       = "DIGEST_INVALID"
	CodeManifestBlobUnknown = "MANIFEST_BLOB_UNKNOWN"
	CodeManifestInvalid     = "MANIFEST_INVALID"
	CodeManifestUnknown     = "MANIFEST_UNKNOWN"
	CodeNameInvalid         = "NAME_INVALID"
	CodeNameUnknown         = "NAME_UNKNOWN"
	CodeSizeInvalid         = "SIZE_INVALID"
	CodeUnauthorized        = "UNAUTHORIZED"
	CodeDenied              = "DENIED"
	CodeUnsupported         = "UNSUPPORTED"
	CodeTooManyRequests     = "TOOMANYREQUESTS"
)

// NewOCIError creates a new OCI error
func NewOCIError(code, message, detail string) *OCIError {
	return &OCIError{
		Code:    code,
		Message: message,
		Detail:  detail,
	}
}

// Error implements the error interface
func (e *OCIError) Error() string {
	return e.Message
}

// NewOCIErrorResponse creates a new OCI error response
func NewOCIErrorResponse(errors ...*OCIError) *OCIErrorResponse {
	errorList := make([]OCIError, len(errors))
	for i, err := range errors {
		errorList[i] = *err
	}
	return &OCIErrorResponse{
		Errors: errorList,
	}
}

// WriteErrorResponse writes an OCI-compliant error response
func WriteErrorResponse(w http.ResponseWriter, statusCode int, errors ...*OCIError) {
	response := NewOCIErrorResponse(errors...)
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	json.NewEncoder(w).Encode(response)
}

// Common error creators
func BlobUnknown(digest string) *OCIError {
	return NewOCIError(CodeBlobUnknown, "blob unknown to registry", digest)
}

func BlobUploadInvalid(detail string) *OCIError {
	return NewOCIError(CodeBlobUploadInvalid, "blob upload invalid", detail)
}

func BlobUploadUnknown(uploadID string) *OCIError {
	return NewOCIError(CodeBlobUploadUnknown, "blob upload unknown to registry", uploadID)
}

func DigestInvalid(digest string) *OCIError {
	return NewOCIError(CodeDigestInvalid, "provided digest did not match uploaded content", digest)
}

func ManifestBlobUnknown(digest string) *OCIError {
	return NewOCIError(CodeManifestBlobUnknown, "manifest references a manifest or blob unknown to registry", digest)
}

func ManifestInvalid(detail string) *OCIError {
	return NewOCIError(CodeManifestInvalid, "manifest invalid", detail)
}

func ManifestUnknown(reference string) *OCIError {
	return NewOCIError(CodeManifestUnknown, "manifest unknown to registry", reference)
}

func NameInvalid(name string) *OCIError {
	return NewOCIError(CodeNameInvalid, "invalid repository name", name)
}

func NameUnknown(name string) *OCIError {
	return NewOCIError(CodeNameUnknown, "repository name not known to registry", name)
}

func SizeInvalid(detail string) *OCIError {
	return NewOCIError(CodeSizeInvalid, "provided length did not match content length", detail)
}

func Unauthorized(detail string) *OCIError {
	return NewOCIError(CodeUnauthorized, "authentication required", detail)
}

func Denied(detail string) *OCIError {
	return NewOCIError(CodeDenied, "requested access to the resource is denied", detail)
}

func Unsupported(detail string) *OCIError {
	return NewOCIError(CodeUnsupported, "the operation is unsupported", detail)
}

func TooManyRequests(detail string) *OCIError {
	return NewOCIError(CodeTooManyRequests, "too many requests", detail)
}