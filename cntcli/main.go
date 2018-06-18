package main

import (
	"flag"
	"log"
	"os"
	"syscall"

	"dockerint/container"
	"dockerint/network"
	"dockerint/registry"
	"dockerint/storage"

	"github.com/docker/docker/pkg/reexec"
)

var containerName = flag.String("n", "", "name of new container [required].")
var storageRootPath = flag.String("d", "", "location of image and container files [optional].")
var imageName = flag.String("i", "", "name of image to run. Docker naming compatible [required].")
var insecureRegistry = flag.Bool("http", false, "If set registry will use http [optional].")
var fsOnly = flag.Bool("o", false, "If set do not start container. Only download and mount FS")
var command = flag.String("c", "/bin/sh", "Command to run")

func init() {
	reexec.Register("nsInit", nsInit)
	if reexec.Init() {
		os.Exit(0)
	}
}

func main() {
	//set proper logging format
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	// parse flags and check if all required info was provided
	flag.Parse()
	if *containerName == "" || *imageName == "" {
		flag.Usage()
		os.Exit(1)
	}

	// initialize storage
	if *storageRootPath != "" {
		storage.SetStorageRootPath(*storageRootPath)
	}
	registry.InsecureRegistry(*insecureRegistry)
	err := storage.InitStorage()
	if err != nil {
		log.Println(err)
	}

	if *fsOnly {
		newRoot, err := container.DownloadAndMount(*imageName, *containerName)
		if err != nil {
			log.Println(err)
			os.Exit(1)
		}
		log.Println("Container root path: ", newRoot)

	} else {

		cmd, newRoot, err := container.SetNameSpaces(*imageName, *containerName, *command)
		if err != nil {
			log.Println(err)
			os.Exit(1)
		}

		// do the clean unmount on exit
		defer unmount(newRoot)

		if err := cmd.Start(); err != nil {
			log.Printf("Error starting the reexec.Command - %s\n", err)
			os.Exit(1)
		}

		log.Println("pid: ", cmd.Process.Pid)
		err = network.Setup(cmd.Process.Pid)
		if err != nil {
			log.Println(err)
			os.Exit(1)
		}

		if err := cmd.Wait(); err != nil {
			log.Printf("Error waiting for the reexec.Command - %s\n", err)
			os.Exit(1)
		}
	}

}

func unmount(path string) error {
	return syscall.Unmount(path, 0)
}
