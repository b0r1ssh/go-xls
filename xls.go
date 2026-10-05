package xls

import (
	"io"
	"os"

	"github.com/b0r1sh/go-xls/internal/cfb"
)

type streamReader interface {
	io.Reader
	io.Seeker
}

type File struct {
	closer io.Closer

	stream streamReader
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
	container, err := cfb.Open(r)
	if err != nil {
		return nil, err
	}

	stream, err := container.Stream("Workbook")
	if err != nil {
		return nil, err
	}

	return &File{
		stream: stream,
	}, nil
}

func (f *File) Close() error {
	if f.closer != nil {
		return f.closer.Close()
	}
	return nil
}
