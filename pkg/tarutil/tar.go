package tarutil

import (
	"archive/tar"
	"bytes"
	"fmt"
	"github.com/pkg/errors"
	"io"
	"os"
)

func ReadAllHeaders(tarFilename string) ([]*tar.Header, error) {

	tarFile, err := os.Open(tarFilename)
	if err != nil {
		return nil, errors.Wrapf(err, "open %s failed", tarFilename)
	}
	defer tarFile.Close()

	reader := tar.NewReader(tarFile)

	var (
		header      *tar.Header
		fileHeaders = make([]*tar.Header, 0)
	)

	for {
		header, err = reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.Wrapf(err, "read tar file")
		}
		fileHeaders = append(fileHeaders, header)

	}
	return fileHeaders, nil
}

func ExtractedByName(tarFilename, extractedName string) ([]byte, error) {

	tarFile, err := os.Open(tarFilename)
	if err != nil {
		return nil, errors.Wrapf(err, "open %s failed", tarFilename)
	}
	defer tarFile.Close()

	reader := tar.NewReader(tarFile)

	var (
		header *tar.Header
	)

	for {
		header, err = reader.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("file %s is not found in %s", extractedName, tarFilename)
		}
		if err != nil {
			return nil, errors.Wrapf(err, "read tar file")
		}
		if header.Name != extractedName {
			continue
		}
		buf := new(bytes.Buffer)

		if _, err = io.Copy(buf, reader); err != nil {
			return nil, errors.Wrapf(err, "read %s failed", extractedName)
		}

		return buf.Bytes(), nil

	}

}
