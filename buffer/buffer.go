package buffer

import "sync"

type Buffer struct {
	cond      sync.Cond
	p         uint64
	fragments map[uint64][]byte
}

func NewBuffer() *Buffer {
	return &Buffer{
		cond: *sync.NewCond(&sync.Mutex{}),

		p:         0,
		fragments: make(map[uint64][]byte),
	}
}

func (b *Buffer) Tell() uint64 {
	b.cond.L.Lock()
	defer b.cond.L.Unlock()
	return b.p
}

func (b *Buffer) Seek(p uint64) {
	b.cond.L.Lock()
	defer b.cond.L.Unlock()

	for {
		buf := b.fragments[b.p]
		size := uint64(len(buf))
		delete(b.fragments, b.p)

		d := p - b.p
		if size == d {
			b.p = p
			return
		} else if size > d {
			b.p = p
			b.fragments[p] = buf[d:]
			return
		} else {
			b.p += size
		}
	}
}

// Note: Will store `buf` directly without coping.
// Don't write to it or the original array anymore.
// Note 2: Overlapped writing was not supported currently.
func (b *Buffer) Write(p uint64, buf []byte) {
	b.cond.L.Lock()
	defer b.cond.L.Unlock()

	b.fragments[p] = buf
	b.cond.Broadcast()
}

// Note: `buf` may still be used internally.
// Don't write to it.
func (b *Buffer) Read(p uint64, nonBlock bool) (buf []byte) {
	b.cond.L.Lock()
	defer b.cond.L.Unlock()

	for {
		buf, ok := b.fragments[p]
		if ok {
			return buf
		}

		if nonBlock {
			return nil
		}

		b.cond.Wait()
	}
}

// Note: `buf` may still be used internally.
// Don't write to it.
func (b *Buffer) Pop(maxSize int, nonBlock bool) (p uint64, buf []byte) {
	b.cond.L.Lock()
	defer b.cond.L.Unlock()

	for {
		p := b.p
		buf, ok := b.fragments[b.p]
		if ok {
			delete(b.fragments, b.p)
			if maxSize < len(buf) {
				b.p += uint64(maxSize)
				b.fragments[b.p] = buf[maxSize:]
				return p, buf[:maxSize]
			} else {
				b.p += uint64(len(buf))
				return p, buf
			}
		}

		if nonBlock {
			return 0, nil
		}

		b.cond.Wait()
	}
}
