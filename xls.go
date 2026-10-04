package xls

import (
	"fmt"
	"io"
	"os"

	"github.com/b0r1sh/go-xls/internal/cfb"
)

type File struct {
	closer io.Closer
}

func OpenFile(path string) (*File, error) {
	o, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	f, err := OpenReader(o)
	if err != nil {
		o.Close()
		return nil, err
	}

	f.closer = o
	return f, nil
}

func OpenReader(r io.ReaderAt) (*File, error) {
	_, err := cfb.Open(r)
	if err != nil {
		return nil, fmt.Errorf("xls: %w", err)
	}

	return &File{}, nil
}

func (f *File) Close() error {
	if f.closer != nil {
		return f.closer.Close()
	}
	return nil
}
