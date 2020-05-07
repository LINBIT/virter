package virter

import "sync"

// a stripped down version of the WriteAtBuffer from
// https://github.com/aws/aws-sdk-go/blob/master/aws/types.go

type writeAtBuffer struct {
	buf []byte
	m   sync.Mutex
}

func newWriteAtBuffer(buf []byte) *writeAtBuffer {
	return &writeAtBuffer{buf: buf}
}

func (b *writeAtBuffer) WriteAt(p []byte, pos int64) (n int, err error) {
	pLen := len(p)
	expLen := pos + int64(pLen)
	b.m.Lock()
	defer b.m.Unlock()
	if int64(len(b.buf)) < expLen {
		if int64(cap(b.buf)) < expLen {
			newBuf := make([]byte, expLen)
			copy(newBuf, b.buf)
			b.buf = newBuf
		}
		b.buf = b.buf[:expLen]
	}
	copy(b.buf[pos:], p)
	return pLen, nil
}

func (b *writeAtBuffer) Bytes() []byte {
	b.m.Lock()
	defer b.m.Unlock()
	return b.buf
}
