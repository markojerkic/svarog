package util

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
)

func ZipFiles(path string, files []string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	zipWriter := zip.NewWriter(file)
	defer zipWriter.Close()

	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()

		fileName := filepath.Base(file)
		writer, err := zipWriter.Create(fileName)

		_, err = io.Copy(writer, f)
		if err != nil {
			return err
		}
	}

	return nil
}

func Unzip(src string, dest string) (string, error) {
	r, err := zip.OpenReader(src)
	if err != nil {
		return "", err
	}
	defer r.Close()

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()

		fpath := filepath.Join(dest, f.Name)

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
		} else {
			if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
				return "", err
			}

			outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return "", err
			}

			_, err = io.Copy(outFile, rc)
			if err != nil {
				return "", err
			}
			outFile.Close()
		}
	}
	return dest, nil
}
