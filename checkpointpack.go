package terra

import (
	"bufio"
	"bytes"
	"compress/flate"
	"encoding/binary"
	"fmt"
	"io"
	"reflect"
	"runtime"
	"sync"
	"unsafe"
)

// How a checkpoint is made small. Its state is a few hundred bytes a tile
// as memory holds it, most of it numbers that change little from one tile
// to the next, and as it lay a globe's was 548 MiB. Three things are done to
// it, none of which the reader has to be told anything to undo:
//
//   - A slice of numbers that is all noughts is written as its length.
//   - A slice that is the very slice an earlier one was - the same memory,
//     as the year's autumn is its spring in the air's budget - is written
//     as which one it was, and is read back as that one, shared again.
//   - The rest are written shuffled: each slice in blocks, and in each
//     block the first byte of every element, then the second, and so on.
//     A field of float64 that varies smoothly has its sign, exponent and
//     high mantissa nearly the same from tile to tile, and laid side by
//     side they compress; interleaved with the low bytes, which are noise,
//     they do not.
//
// And then the whole is compressed (packWriter): deflate at its fastest, in
// blocks compressed side by side.

const (
	packBytes = iota // the elements follow, shuffled
	packZero         // all noughts
	packSame         // the same slice as the one numbered next
)

// shuffleBlock is how many elements of a slice are shuffled together.
const shuffleBlock = 1 << 16

// packedAt is a slice as the packing knows it: where it lies, how long it
// is and its type. Two slices with all three alike are the same slice.
type packedAt struct {
	at  unsafe.Pointer
	n   int
	typ reflect.Type
}

// packSlice writes v, a slice of numbers whose length is written, packed.
func (c *historyCodec) packSlice(v reflect.Value) {
	if c.err != nil {
		return
	}
	n, size := v.Len(), int(v.Type().Elem().Size())
	b := raw(v.UnsafePointer(), uintptr(n*size))
	if len(b) == 0 {
		c.uint(packBytes)
		return
	}
	key := packedAt{v.UnsafePointer(), n, v.Type()}
	if k, ok := c.seen[key]; ok {
		c.uint(packSame)
		c.uint(k)
		return
	}
	if c.seen == nil {
		c.seen = make(map[packedAt]uint64)
	}
	c.seen[key] = uint64(len(c.seen))
	if allNought(b) {
		c.uint(packZero)
		return
	}
	c.uint(packBytes)
	if size == 1 {
		_, c.err = c.w.Write(b)
		return
	}
	// A few blocks are shuffled at once, side by side, and then written in
	// their order.
	for off := 0; off < n && c.err == nil; off += shuffleBlock * shuffleSide {
		end := min(n, off+shuffleBlock*shuffleSide)
		c.shuf = grow(c.shuf, (end-off)*size)
		buf := c.shuf[:(end-off)*size]
		sideBySide(off, end, func(lo, hi int) {
			shuffle(buf[(lo-off)*size:(hi-off)*size], b[lo*size:hi*size], size)
		})
		_, c.err = c.w.Write(buf)
	}
}

// shuffleSide is how many blocks of a slice are shuffled at once.
const shuffleSide = 8

// sideBySide calls f on each block of the elements from lo to hi, each on a
// goroutine of its own where there is more than one.
func sideBySide(lo, hi int, f func(lo, hi int)) {
	if hi-lo <= shuffleBlock {
		f(lo, hi)
		return
	}
	var wg sync.WaitGroup
	for at := lo; at < hi; at += shuffleBlock {
		wg.Add(1)
		go func(at int) {
			defer wg.Done()
			f(at, min(hi, at+shuffleBlock))
		}(at)
	}
	wg.Wait()
}

// unpackSlice reads into v, a slice of numbers n long, what packSlice wrote.
func (c *historyCodec) unpackSlice(v reflect.Value, n int, path string) {
	t := v.Type()
	how := c.readUint()
	if c.err != nil {
		return
	}
	if how == packSame {
		k := c.readUint()
		if c.err == nil && (k >= uint64(len(c.read)) || c.read[k].Type() != t || c.read[k].Len() != n) {
			c.err = fmt.Errorf("%w: %s is the same as no slice before it", ErrHistoryFile, path)
		}
		if c.err == nil {
			v.Set(c.read[k])
		}
		return
	}
	if how != packBytes && how != packZero {
		c.err = fmt.Errorf("%w: %s is packed as %d", ErrHistoryFile, path, how)
		return
	}
	s := reflect.MakeSlice(t, n, n)
	v.Set(s)
	size := int(t.Elem().Size())
	if n == 0 || size == 0 {
		return
	}
	c.read = append(c.read, s)
	if how == packZero {
		return
	}
	b := raw(s.UnsafePointer(), uintptr(n*size))
	if size == 1 {
		c.fill(b)
		return
	}
	for off := 0; off < n && c.err == nil; off += shuffleBlock * shuffleSide {
		end := min(n, off+shuffleBlock*shuffleSide)
		c.shuf = grow(c.shuf, (end-off)*size)
		buf := c.shuf[:(end-off)*size]
		c.fill(buf)
		if c.err != nil {
			return
		}
		sideBySide(off, end, func(lo, hi int) {
			unshuffle(b[lo*size:hi*size], buf[(lo-off)*size:(hi-off)*size], size)
		})
	}
}

// shuffle lays the elements of seg, size bytes each, into to byte by byte:
// the first byte of every element, then the second, and so on. It goes a
// run of shuffleRun elements at a time, which a cache holds, so that each
// byte's run is written a whole line at once: the runs of a block lie a
// power of two apart, and written an element at a time they fight over the
// same few lines of the cache.
func shuffle(to, seg []byte, size int) {
	m := len(seg) / size
	for i0 := 0; i0 < m; i0 += shuffleRun {
		i1 := min(m, i0+shuffleRun)
		from := seg[i0*size : i1*size]
		for k := range size {
			run := to[k*m+i0 : k*m+i1]
			for j := range run {
				run[j] = from[j*size+k]
			}
		}
	}
}

// unshuffle is shuffle undone: the elements of seg put back from from.
func unshuffle(seg, from []byte, size int) {
	m := len(seg) / size
	for i0 := 0; i0 < m; i0 += shuffleRun {
		i1 := min(m, i0+shuffleRun)
		to := seg[i0*size : i1*size]
		for k := range size {
			run := from[k*m+i0 : k*m+i1]
			for j, x := range run {
				to[j*size+k] = x
			}
		}
	}
}

// shuffleRun is how many elements shuffle takes at a time.
const shuffleRun = 64

// dropped reports whether field i of t is one the codec leaves out.
func (c *historyCodec) dropped(t reflect.Type, i int) bool {
	if c.drop == nil {
		return false
	}
	_, ok := c.drop[t.Name()+"."+t.Field(i).Name]
	return ok
}

// grow is b with room for n bytes.
func grow(b []byte, n int) []byte {
	if cap(b) < n {
		return make([]byte, n)
	}
	return b
}

// allNought reports whether every byte of b is nought.
func allNought(b []byte) bool {
	for len(b) >= 8 {
		if binary.LittleEndian.Uint64(b) != 0 {
			return false
		}
		b = b[8:]
	}
	for _, x := range b {
		if x != 0 {
			return false
		}
	}
	return true
}

// packBlock is how much of the stream is compressed as one piece. The
// pieces are compressed side by side, and each is a deflate stream of its
// own, so where one ends is fixed by the stream and not by how many
// goroutines there were: the file is the same whatever the machine.
const packBlock = 4 << 20

// A packWriter compresses what is written to it in packBlock pieces, each
// written as its length, its compressed length and then the compressed
// bytes, and ended by a nought. Close writes the last piece and the end.
type packWriter struct {
	out     io.Writer
	buf     []byte
	pending chan chan packed // the pieces being compressed, in order
	done    chan error
}

type packed struct {
	raw int
	b   []byte
	err error
}

func newPackWriter(out io.Writer) *packWriter {
	p := &packWriter{
		out:     out,
		buf:     make([]byte, 0, packBlock),
		pending: make(chan chan packed, runtime.GOMAXPROCS(0)),
		done:    make(chan error, 1),
	}
	go func() {
		var err error
		var head [2 * binary.MaxVarintLen64]byte
		for r := range p.pending {
			got := <-r
			if err != nil {
				continue
			}
			if err = got.err; err != nil {
				continue
			}
			k := binary.PutUvarint(head[:], uint64(got.raw))
			k += binary.PutUvarint(head[k:], uint64(len(got.b)))
			if _, err = p.out.Write(head[:k]); err == nil {
				_, err = p.out.Write(got.b)
			}
		}
		if err == nil {
			_, err = p.out.Write([]byte{0})
		}
		p.done <- err
	}()
	return p
}

func (p *packWriter) Write(b []byte) (int, error) {
	n := len(b)
	for len(b) > 0 {
		k := min(len(b), packBlock-len(p.buf))
		p.buf = append(p.buf, b[:k]...)
		b = b[k:]
		if len(p.buf) == packBlock {
			p.send()
		}
	}
	return n, nil
}

// send hands the piece in buf to a goroutine of its own to compress.
func (p *packWriter) send() {
	r := make(chan packed, 1)
	p.pending <- r
	go func(b []byte) {
		var out bytes.Buffer
		out.Grow(len(b) / 2)
		w, err := flate.NewWriter(&out, flate.BestSpeed)
		if err == nil {
			_, err = w.Write(b)
		}
		if err == nil {
			err = w.Close()
		}
		r <- packed{raw: len(b), b: out.Bytes(), err: err}
	}(p.buf)
	p.buf = make([]byte, 0, packBlock)
}

// Close compresses and writes what is left, and the end.
func (p *packWriter) Close() error {
	if len(p.buf) > 0 {
		p.send()
	}
	close(p.pending)
	return <-p.done
}

// A packReader reads what a packWriter wrote, the pieces read ahead and
// decompressed side by side.
type packReader struct {
	pending chan chan packed
	cur     []byte
	err     error
	stop    chan struct{}
}

func newPackReader(in *bufio.Reader) *packReader {
	p := &packReader{
		pending: make(chan chan packed, runtime.GOMAXPROCS(0)),
		stop:    make(chan struct{}),
	}
	go func() {
		defer close(p.pending)
		for {
			r := make(chan packed, 1)
			n, err := binary.ReadUvarint(in)
			if err == nil && n == 0 {
				return
			}
			var size uint64
			if err == nil {
				size, err = binary.ReadUvarint(in)
			}
			if err == nil && (n > packBlock || size > packBlock+packBlock/8+1<<10) {
				err = fmt.Errorf("a piece of %d bytes packed in %d", n, size)
			}
			var b []byte
			if err == nil {
				b = make([]byte, size)
				_, err = io.ReadFull(in, b)
			}
			if err != nil {
				r <- packed{err: fmt.Errorf("%w: %v", ErrHistoryFile, err)}
			} else {
				go func() {
					out := make([]byte, n)
					_, err := io.ReadFull(flate.NewReader(bytes.NewReader(b)), out)
					if err != nil {
						err = fmt.Errorf("%w: a piece: %v", ErrHistoryFile, err)
					}
					r <- packed{raw: int(n), b: out, err: err}
				}()
			}
			select {
			case p.pending <- r:
			case <-p.stop:
				return
			}
			if err != nil {
				return
			}
		}
	}()
	return p
}

func (p *packReader) Read(b []byte) (int, error) {
	for len(p.cur) == 0 {
		if p.err != nil {
			return 0, p.err
		}
		r, ok := <-p.pending
		if !ok {
			p.err = io.EOF
			continue
		}
		got := <-r
		p.cur, p.err = got.b, got.err
	}
	n := copy(b, p.cur)
	p.cur = p.cur[n:]
	return n, nil
}

// Close lets the reading ahead go.
func (p *packReader) Close() {
	close(p.stop)
	for range p.pending {
	}
}
