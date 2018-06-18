package storage

import (
	"archive/tar"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/odk-/dockerinternals/registry"

	"golang.org/x/sys/unix"
)

// AufsDeletedMark files beginning with this will be unavailable in upper layers
// used in checkIfDeleted() at the bottom
const AufsDeletedMark = ".wh."

// AufsDeletedDirMark dirs with this file are unavailable in upper layers
// used in checkIfDeleted() at the bottom
const AufsDeletedDirMark = ".wh..wh..opq"

var (
	storageRootPath = "/tmp/cme"
)

// SetStorageRootPath configures root path used to store all data. Defaults to /tmp/cme
func SetStorageRootPath(path string) {
	storageRootPath = path
}

// InitStorage checks if proper folder structure is present and creates it if needed
/*
proper structure looks like this:
-storageRootPath
|-manifests			<- jsons with name as base64 string from: registry URI + image name + tag
|-blobs				<- image layers
|-containers			<- containers will have their fs here
||-<container_name>
|||-rootfs			<- mounted overlayfs
|||-workdir			<- working layer used internally by overlay
|||-upper			<- top layer that will hold all changes to image
*/
func InitStorage() error {
	err := os.Mkdir(storageRootPath, 0755)
	if err != nil && os.IsNotExist(err) {
		return err
	}
	err = os.Mkdir(storageRootPath+"/manifests", 0755)
	if err != nil && os.IsNotExist(err) {
		return err
	}
	err = os.Mkdir(storageRootPath+"/blobs", 0755)
	if err != nil && os.IsNotExist(err) {
		return err
	}
	err = os.Mkdir(storageRootPath+"/containers", 0755)
	if err != nil && os.IsNotExist(err) {
		return err
	}
	return nil
}

// CreateContainerRootFS sets up root fs for container and returns path to it
// it will download layers from registry if needed
func CreateContainerRootFS(img *registry.Image, containerName string) (string, error) {
	var (
		manifest *registry.DockerManifest
		err      error
	)
	manifest, err = loadManifest(img)
	if err != nil {
		// load from disk failed we need to download manifest and store it for future
		manifest, err = registry.GetManifest(img)
		if err != nil {
			return "", err
		}
		err = saveManifest(manifest, img)
		if err != nil {
			log.Println("Manifest save failed: ", err)
		}
	}
	//iterate over layers from manifest
	for _, layer := range manifest.Layers {
		if layer.MediaType != registry.MediaTypeLayer {
			return "", fmt.Errorf("Layer media type (%s) unsupported. For now only %s is supported", layer.MediaType, registry.MediaTypeLayer)
		}
		if !checkLayerPresence(layer.Digest) {
			err = downloadLayer(img, layer.Digest, layer.MediaType)
			if err != nil {
				return "", err
			}
		}
	}
	// most important part, this actually mounts merged overlay filesystem
	containerPath := filepath.Join(storageRootPath, "containers", containerName)
	err = mountImageOverlay(manifest, containerPath)
	if err != nil {
		return "", nil
	}
	return filepath.Join(containerPath, "rootfs"), nil
}

// SaveManifest stores manifests on disk to speed up starting new containers
func saveManifest(manifest *registry.DockerManifest, img *registry.Image) error {
	f, err := os.OpenFile(storageRootPath+"/manifests/"+generateJSONName(img), os.O_CREATE|os.O_WRONLY, 0644)
	defer f.Close()
	if err != nil {
		return err
	}
	// prepare JSON content
	j, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	// save it to disk
	_, err = f.Write(j)
	if err != nil {
		return err
	}
	return nil
}

// LoadManifest returns manifest from disk. Error if not present
func loadManifest(img *registry.Image) (*registry.DockerManifest, error) {
	file, err := ioutil.ReadFile(storageRootPath + "/manifests/" + generateJSONName(img))
	if err != nil {
		return nil, err
	}
	var manifest = &registry.DockerManifest{}
	err = json.Unmarshal(file, manifest)
	if err != nil {
		return nil, err
	}
	return manifest, nil
}

func generateJSONName(img *registry.Image) string {
	return base64.StdEncoding.EncodeToString([]byte(img.Registry+img.ImageName+img.Tag)) + ".json"
}

// checkLayerPresence checks if layer is already on disk
// it will check only dir presence, won't check integrity
func checkLayerPresence(digest string) bool {
	if _, err := os.Stat(filepath.Join(storageRootPath, "blobs", digest)); err != nil {
		return false
	}
	return true
}

/*
 downloadLayer gets layer from registry and unpacks it.
 This is a very important function and quite big one.
 Normally I would divide it into smaller ones but this time it will be
 more readable when pasted into blog post. Docker supports different
 compression formats on images but here we support only tar.gz
*/
func downloadLayer(img *registry.Image, digest string, compression string) error {
	// download blob from registry
	blob, err := registry.GetBlob(img, digest)
	if err != nil {
		return err
	}
	// start unpacking
	gz, err := gzip.NewReader(blob)
	tr := tar.NewReader(gz)
	log.Printf("Downloading and unpacking layer: %s\n", digest)
	// create directory for layer
	if err := os.MkdirAll(filepath.Join(storageRootPath, "blobs", digest), 0755); err != nil {
		return err
	}
	// handle each file from layer
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			// end of tar archive
			break
		}
		if err != nil {
			return err
		}
		// set destination path for the file
		dst := filepath.Join(storageRootPath, "blobs", digest, hdr.Name)

		// here we check type of each tar element and handle it in proper way
		switch hdr.Typeflag {
		case tar.TypeDir:
			if _, err := os.Stat(dst); err != nil {
				if err := os.MkdirAll(dst, 0755); err != nil {
					return err
				}
			}
		case tar.TypeReg:
			// regular file, can be possibly AUFS deletion (whiteout) mark
			// docker saves info about deleted files in AUFS format so we need to convert it into overlay ones while unpacking
			// AUFS format is file based that is why we check that only on regular file type
			writeFile, err := checkIfDeleted(hdr, dst)
			if err != nil {
				return err
			}
			if !writeFile {
				continue
			}
			//not a del mark, just create the file
			f, err := os.OpenFile(dst, os.O_CREATE|os.O_RDWR, os.FileMode(hdr.Mode))
			defer f.Close()
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				return err
			}
			f.Close()
		case tar.TypeSymlink:
			err := os.Symlink(hdr.Linkname, dst)
			if err != nil {
				return err
			}
		case tar.TypeLink:
			// very naive security to not have hard links that point outside of container
			if !strings.HasPrefix(dst, storageRootPath+"/blobs") {
				return fmt.Errorf("invalid hardlink %q -> %q", dst, hdr.Linkname)
			}
			err := os.Link(filepath.Join(storageRootPath, "blobs", digest, hdr.Linkname), dst)
			if err != nil {
				return err
			}
		default:
			//we don't handle this type of file, final mounted image may be broken but I didn't found any that gets that far
			//safe to ignore, feel free to add more types if you need some
			fmt.Printf("Unsupported file type found; name: %s\tmode: %v\tdigest: %s\ttarget: %s\n", hdr.Name, hdr.Typeflag, digest, dst)
		}
	}
	return nil
}

/*
 Based on https://github.com/moby/moby/blob/54251b53d7081f1471c0a2cb23fd4e198e71bd14/pkg/archive/archive_linux.go#L64
 In short this function looks for AUFS marks that file or dir was deleted in that layer so future calls won't show it.
 If such mark is found its get converted to OverlayFs one
*/
func checkIfDeleted(hdr *tar.Header, dst string) (bool, error) {
	// get filename and path to it
	fileName := filepath.Base(dst)
	filePath := filepath.Dir(dst)

	// if a directory is marked as opaque by the AUFS special file, we need to translate that to overlay
	if fileName == AufsDeletedDirMark {
		err := unix.Setxattr(filePath, "trusted.overlay.opaque", []byte{'y'}, 0)
		// don't write the file itself. It would show up in merged dir.
		return false, err
	}

	// if a file was deleted and we are using overlay, we need to create a character device as defined in overlay man
	if strings.HasPrefix(fileName, AufsDeletedMark) {
		originalBase := fileName[len(AufsDeletedMark):]
		originalPath := filepath.Join(filePath, originalBase)

		if err := unix.Mknod(originalPath, unix.S_IFCHR, 0); err != nil {
			return false, err
		}
		if err := os.Chown(originalPath, hdr.Uid, hdr.Gid); err != nil {
			return false, err
		}

		// don't write the file itself, we created char device in its place
		return false, nil
	}

	return true, nil
}
