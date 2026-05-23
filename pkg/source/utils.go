package source

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"sort"
)

type bytesWriteBuffer struct {
	buf []byte
}

func (b *bytesWriteBuffer) Write(p []byte) (int, error) {
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *bytesWriteBuffer) Bytes() []byte {
	return b.buf
}

func packTarGz(files map[string][]byte) ([]byte, error) {
	bufWriter := &bytesWriteBuffer{}
	gw := gzip.NewWriter(bufWriter)
	tw := tar.NewWriter(gw)

	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		hdr := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(files[name])),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, fmt.Errorf("tar write header: %w", err)
		}
		if _, err := tw.Write(files[name]); err != nil {
			return nil, fmt.Errorf("tar write: %w", err)
		}
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("tar close: %w", err)
	}
	if err := gw.Close(); err != nil {
		return nil, fmt.Errorf("gzip close: %w", err)
	}

	return bufWriter.Bytes(), nil
}
