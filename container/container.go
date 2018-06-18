package container

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/odk-/dockerinternals/registry"
	"github.com/odk-/dockerinternals/storage"

	"github.com/docker/docker/pkg/reexec"
)

//DownloadAndMount invoke image download and mount of image filesystem.
func DownloadAndMount(imageName, containerName string) (string, error) {
	registry.SetDefaultRegistry("registry-1.docker.io")
	img, err := registry.ParseImageName(imageName)
	if err != nil {
		return "", err
	}
	rootPath, err := storage.CreateContainerRootFS(img, containerName)
	if err != nil {
		return "", err
	}
	return rootPath, nil
}

//SetNameSpaces sets all required namespaces for the process and execute fork
func SetNameSpaces(imageName, containerName, command string) (*exec.Cmd, string, error) {
	newRoot, err := DownloadAndMount(imageName, containerName)
	if err != nil {
		return nil, "", err
	}

	/*
	*
	* Here should be possible to drop root privileges and run in usermode NS directly
	*
	 */

	cmd := reexec.Command("nsInit", newRoot, command)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWNS |
			syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWIPC |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWNET |
			syscall.CLONE_NEWUSER,
		UidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getuid(),
				Size:        1,
			},
		},
		GidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getgid(),
				Size:        1,
			},
		},
	}
	return cmd, newRoot, nil
}
