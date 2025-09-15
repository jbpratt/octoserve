package types

import (
	"encoding/json"
	"time"
)

// Manifest represents an OCI manifest
type Manifest struct {
	MediaType     string            `json:"mediaType"`
	SchemaVersion int               `json:"schemaVersion"`
	Config        Descriptor        `json:"config,omitempty"`
	Layers        []Descriptor      `json:"layers,omitempty"`
	Manifests     []Descriptor      `json:"manifests,omitempty"` // For image index
	Subject       *Descriptor       `json:"subject,omitempty"`
	Annotations   map[string]string `json:"annotations,omitempty"`

	// Internal fields
	Digest     string    `json:"-"`
	Size       int64     `json:"-"`
	Repository string    `json:"-"`
	Tag        string    `json:"-"`
	CreatedAt  time.Time `json:"-"`
}

// ImageIndex represents an OCI image index for referrers API
type ImageIndex struct {
	SchemaVersion int               `json:"schemaVersion"`
	MediaType     string            `json:"mediaType"`
	Manifests     []Descriptor      `json:"manifests"`
	Annotations   map[string]string `json:"annotations,omitempty"`
}

// Descriptor represents a content descriptor
type Descriptor struct {
	MediaType    string            `json:"mediaType"`
	Digest       string            `json:"digest"`
	Size         int64             `json:"size"`
	URLs         []string          `json:"urls,omitempty"`
	Annotations  map[string]string `json:"annotations,omitempty"`
	Data         []byte            `json:"data,omitempty"`
	Platform     *Platform         `json:"platform,omitempty"`
	ArtifactType string            `json:"artifactType,omitempty"`
}

// Platform represents platform-specific information
type Platform struct {
	Architecture string   `json:"architecture"`
	OS           string   `json:"os"`
	OSVersion    string   `json:"os.version,omitempty"`
	OSFeatures   []string `json:"os.features,omitempty"`
	Variant      string   `json:"variant,omitempty"`
}

// MediaTypes contains common OCI media types
var MediaTypes = struct {
	ImageManifest    string
	ImageIndex       string
	ImageConfig      string
	ImageLayer       string
	ImageLayerGzip   string
	ImageLayerZstd   string
	ArtifactManifest string
}{
	ImageManifest:    "application/vnd.oci.image.manifest.v1+json",
	ImageIndex:       "application/vnd.oci.image.index.v1+json",
	ImageConfig:      "application/vnd.oci.image.config.v1+json",
	ImageLayer:       "application/vnd.oci.image.layer.v1.tar",
	ImageLayerGzip:   "application/vnd.oci.image.layer.v1.tar+gzip",
	ImageLayerZstd:   "application/vnd.oci.image.layer.v1.tar+zstd",
	ArtifactManifest: "application/vnd.oci.artifact.manifest.v1+json",
}

// ParseManifest parses a manifest from JSON bytes
func ParseManifest(data []byte) (*Manifest, error) {
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	manifest.Size = int64(len(data))
	return &manifest, nil
}

// MarshalJSON marshals the manifest to JSON
func (m *Manifest) MarshalJSON() ([]byte, error) {
	type Alias Manifest
	return json.Marshal((*Alias)(m))
}
