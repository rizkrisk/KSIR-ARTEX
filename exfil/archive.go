package exfil

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// PackageArchive packs artifacts into a .tar.gz in the task directory.
func PackageArchive(taskID int64, artifacts []Artifact, destDir string) (string, error) {
	archivePath := filepath.Join(destDir, fmt.Sprintf("exfil-task-%d-%d.tar.gz", taskID, time.Now().Unix()))
	f, err := os.Create(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	for _, a := range artifacts {
		if err := addToTar(tw, a.Path, a.StoredPath); err != nil {
			continue // skip unreadable files
		}
	}
	return archivePath, nil
}

func addToTar(tw *tar.Writer, srcPath, displayPath string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	hdr := &tar.Header{
		Name:    filepath.Base(displayPath),
		Size:    info.Size(),
		Mode:    int64(info.Mode()),
		ModTime: info.ModTime(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err = io.Copy(tw, f)
	return err
}
