package storage

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"dockerint/registry"
)

/*
 mounts all layers from manifest into target path
 new workdir is created for container
 more about overlay: https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/tree/Documentation/filesystems/overlayfs.txt?h=v4.13
 more about mount syscall: man 2 mount
*/
func mountImageOverlay(manifest *registry.DockerManifest, target string) error {
	err := createMountTargetDirs(target)
	if err != nil {
		return err
	}
	// get layer locations converted into mount options
	mountOptions := prepareOverlayMountOptions(manifest, target)

	// we could do os.exec here and use mount command, but golang have syscall support so os.exec feels like a cheat :)
	err = syscall.Mount("overlay", filepath.Join(target, "rootfs"), "overlay", 0, mountOptions)
	if err != nil {
		return err
	}
	return nil
}

// just ensure that target directories are there
func createMountTargetDirs(target string) error {
	if err := os.MkdirAll(filepath.Join(target, "rootfs"), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(target, "workdir"), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(target, "upper"), 0755); err != nil {
		return err
	}
	return nil
}

/*
Prepare mount options string. Probably not cleanest way to do it but hey: It works!
strings.Replace was used to escape ":" in digest. Otherwise it would break mount params.
In short we just append all digests with their paths into array so we can do join later and don't worry about ":" at the end of string
also from overlay manual:
	...
	mount -t overlay overlay -olowerdir=/lower1:/lower2:/lower3 /merged
	...
	The specified lower directories will be stacked beginning from the
	rightmost one and going left.  In the above example lower1 will be the
	top, lower2 the middle and lower3 the bottom layer.

So we need to revert layer order from manifest
*/
func prepareOverlayMountOptions(manifest *registry.DockerManifest, target string) string {
	var (
		digests []string
		lowers  string
	)

	for i := len(manifest.Layers) - 1; i >= 0; i-- {
		digests = append(digests, strings.Replace(filepath.Join(storageRootPath, "blobs", manifest.Layers[i].Digest), ":", "\\:", 1))
	}
	lowers = strings.Join(digests, ":")
	return "lowerdir=" + lowers + ",upperdir=" + filepath.Join(target, "upper") + ",workdir=" + filepath.Join(target, "workdir")
}
