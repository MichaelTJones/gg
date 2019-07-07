package main

import (
	"archive/tar"
	"archive/zip"
	"errors"
	"io"

	"github.com/cavaliercoder/go-cpio"
)

// these are the allowed extensions in the multiReader
const (
	eCPIO = iota
	eTAR
	eZIP
)

// multiReader is a struct to allow us to treat all files
// the same way. It implements the ReadNexter interface.
// Every multiReader can have a single implementation
// inside, a zip multiReader cannot be used to read tar files.
type multiReader struct {
	ext   int
	rCPIO *cpio.Reader
	rTAR  *tar.Reader

	rZIP      *zip.ReadCloser
	zipReader io.Reader
	// zipIndex needs to start the value -1, otherwise
	// our logic to determine wich file we are reading
	// will not work
	zipIndex int
}

func (r *multiReader) Read(p []byte) (int, error) {
	switch r.ext {
	case eCPIO:
		return r.rCPIO.Read(p)
	case eTAR:
		return r.rTAR.Read(p)
	case eZIP:
		n, e := r.zipReader.Read(p)
		return n, e
	}
	return 0, errors.New("internal reader not found")
}

func (r *multiReader) Next() (string, error) {
	switch r.ext {
	case eCPIO:
		header, err := r.rCPIO.Next()
		n := ""
		if err == nil {
			n = header.Name
		}
		return n, err
	case eTAR:
		header, err := r.rTAR.Next()
		n := ""
		if err == nil {
			n = header.Name
		}
		return n, err
	case eZIP:
		r.zipIndex++
		if r.zipIndex >= len(r.rZIP.Reader.File) {
			r.rZIP.Close()
			return "", io.EOF
		}

		file := r.rZIP.Reader.File[r.zipIndex]
		reader, err := file.Open()
		if err != nil {
			return "", err
		}
		r.zipReader = reader
		f := file.FileHeader.Name

		return f, nil
	}
	return "", errors.New("internal reader not found")
}

func newMultiReader(r io.Reader, ext string, name string) *multiReader {
	switch ext {
	case ".cpio":
		final := cpio.NewReader(r)
		return &multiReader{ext: eCPIO, rCPIO: final}
	case ".tar":
		tr := tar.NewReader(r)
		return &multiReader{ext: eTAR, rTAR: tr}
	case ".zip":
		z, err := zip.OpenReader(name)
		if err != nil {
			println(err)
			return &multiReader{}
		}
		return &multiReader{ext: eZIP, rZIP: z, zipIndex: -1}
	}
	return &multiReader{}
}
