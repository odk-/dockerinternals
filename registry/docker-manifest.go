package registry

// here we will define struct for docker manifest file and constants for media types

// DockerManifest holds structure for parsed manifest json
type DockerManifest struct {
	//this needs to be 2 as we don't support other versions here
	SchemaVersion int `json:"schemaVersion"`

	MediaType string `json:"mediaType,omitempty"`

	Config imageConfig `json:"config"`

	Layers []imageLayers `json:"layers"`
}

type imageConfig struct {
	MediaType string `json:"mediaType,omitempty"`
	Size      int    `json:"size,omitempty"`
	Digest    string `json:"digest"`
}

type imageLayers struct {
	MediaType string `json:"mediaType,omitempty"`
	Size      int    `json:"size,omitempty"`
	Digest    string `json:"digest"`
}

// this part is from docker code itself
const (
	// MediaTypeManifest specifies the mediaType for the current version.
	MediaTypeManifest = "application/vnd.docker.distribution.manifest.v2+json"

	// MediaTypeImageConfig specifies the mediaType for the image configuration.
	MediaTypeImageConfig = "application/vnd.docker.container.image.v1+json"

	// MediaTypePluginConfig specifies the mediaType for plugin configuration.
	MediaTypePluginConfig = "application/vnd.docker.plugin.v1+json"

	// MediaTypeLayer is the mediaType used for layers referenced by the
	// manifest.
	MediaTypeLayer = "application/vnd.docker.image.rootfs.diff.tar.gzip"

	// MediaTypeForeignLayer is the mediaType used for layers that must be
	// downloaded from foreign URLs.
	MediaTypeForeignLayer = "application/vnd.docker.image.rootfs.foreign.diff.tar.gzip"

	// MediaTypeUncompressedLayer is the mediaType used for layers which
	// are not compressed.
	MediaTypeUncompressedLayer = "application/vnd.docker.image.rootfs.diff.tar"
)
