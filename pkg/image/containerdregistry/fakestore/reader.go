package fakestore

import (
	"io"
)

type FakeReader struct {
	buf []byte
}

func (r *FakeReader) ReadAt(p []byte, off int64) (n int, err error) {
	if int64(len(r.buf)) < off {
		return 0, nil
	}
	copy(p, r.buf[off:])
	if len(p) > len(r.buf)-int(off) {
		return len(r.buf) - int(off), io.EOF
	}
	return len(r.buf) - int(off), nil
}

func (r *FakeReader) Close() error {
	return nil
}

func (r *FakeReader) Size() int64 {
	return int64(len(r.buf))
}
