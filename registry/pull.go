package registry

import (
	"encoding/json"
	"io"
	"net/http"
)

var (
	err error
)

// GetManifest download image manifest from registry
func GetManifest(img *Image) (*DockerManifest, error) {
	//docker registry might require auth token for pulling manifests. If token is not set, try to get it.
	if img.Token == "" {
		err = img.getAuthToken()
	}
	if err != nil {
		return nil, err
	}
	client := &http.Client{}
	req, err := http.NewRequest("GET", protocol+"://"+img.Registry+"/v2/"+img.ImageName+"/manifests/"+img.Tag, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	req.Header.Add("Authorization", "Bearer "+img.Token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	var manifest DockerManifest
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&manifest)
	if err != nil {
		return nil, err
	}
	return &manifest, nil
}

// GetBlob downloads compressed layer and returns it as byte stream for further processing in storage package
func GetBlob(img *Image, digest string) (io.ReadCloser, error) {
	if img.Token == "" {
		err = img.getAuthToken()
	}
	client := &http.Client{}
	req, err := http.NewRequest("GET", protocol+"://"+img.Registry+"/v2/"+img.ImageName+"/blobs/"+digest, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+img.Token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}
