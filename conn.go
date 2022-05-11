package l4la

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"

	"github.com/hzyitc/go-notify"
	"github.com/hzyitc/l4la/buffer"
	"github.com/hzyitc/mnh/log"
)

type Conn struct {
	local net.Conn

	ctx    context.Context
	cancel context.CancelFunc

	remotesLock   sync.Mutex
	remotes       []net.Conn
	remotesNotify notify.Notify

	read     uint64
	readBuf  *buffer.Buffer
	readLock sync.Mutex

	selected int
	write    uint64
}

var byteOrder = binary.BigEndian

type header struct {
	Offset uint64
	Size   uint16
}

func NewConn(ctx context.Context, local net.Conn) (*Conn, error) {
	ctx, cancel := context.WithCancel(ctx)

	c := &Conn{
		local: local,

		ctx:    ctx,
		cancel: cancel,

		remotesLock:   sync.Mutex{},
		remotes:       nil,
		remotesNotify: notify.Notify{},

		read:     0,
		readBuf:  buffer.NewBuffer(),
		readLock: sync.Mutex{},

		selected: 0,
		write:    0,
	}

	go c.main()

	return c, nil
}

func (c *Conn) Close() {
	c.cancel()
}

func (c *Conn) AddRemoteConn(conn net.Conn) {
	c.remotesLock.Lock()
	select {
	case <-c.ctx.Done():
		c.remotesLock.Unlock()
		conn.Close()
		return
	default:
	}
	c.remotes = append(c.remotes, conn)
	c.remotesLock.Unlock()

	go func() {
		c.handleRemote(c.ctx, conn)
		c.cancel()
	}()

	c.remotesNotify.NotifyAll()
}

func (c *Conn) handleRemote(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	for {
		var h header
		err := binary.Read(conn, byteOrder, &h)
		if err != nil {
			log.Error("handleRemote read header error:", err.Error())
			return
		}

		buf := make([]byte, h.Size)
		_, err = io.ReadFull(conn, buf)
		if err != nil {
			log.Error("handleRemote read error:", err.Error())
			return
		}

		c.readBuf.Write(h.Offset, buf)
		atomic.AddUint64(&c.read, uint64(h.Size))

		if !func() bool {
			c.readLock.Lock()
			defer c.readLock.Unlock()

			for {
				_, buf = c.readBuf.Pop(9000, true)
				if buf == nil {
					return true
				}

				n, err := c.local.Write(buf)
				if err != nil {
					log.Error("handleRemote write error:", err.Error())
					return false
				}
				if n != len(buf) {
					log.Error("handleRemote write error:", fmt.Sprintf("sent %d bytes instand of %d bytes", n, len(buf)))
					return false
				}
			}
		}() {
			return
		}
	}
}

func (c *Conn) handleLocal(ctx context.Context) {
	defer c.local.Close()

	buf := make([]byte, 9000)
	for {
		size, err := c.local.Read(buf)
		if err != nil {
			log.Error("handleLocal error:", err.Error())
			return
		}

		c.remotesLock.Lock()
		conn := c.remotes[c.selected]
		c.selected = (c.selected + 1) % len(c.remotes)
		c.remotesLock.Unlock()

		err = binary.Write(conn, byteOrder, header{
			Offset: c.write,
			Size:   uint16(size),
		})
		if err != nil {
			log.Error("handleLocal write header error:", err.Error())
			return
		}

		n, err := conn.Write(buf[:size])
		if err != nil {
			log.Error("handleLocal write error:", err.Error())
			return
		}
		if n != size {
			log.Error("handleLocal write error:", fmt.Errorf("sent %d bytes instand of %d bytes", n, len(buf[:16+size])))
			return
		}

		c.write += uint64(size)
	}
}

func (c *Conn) main() {
	for {
		ch := c.remotesNotify.Wait()

		c.remotesLock.Lock()
		if len(c.remotes) >= 2 {
			c.remotesLock.Unlock()
			break
		}
		c.remotesLock.Unlock()

		select {
		case <-c.ctx.Done():
			return
		case <-ch:
		}
	}

	go func() {
		c.handleLocal(c.ctx)
		c.cancel()
	}()

	<-c.ctx.Done()
	c.remotesLock.Lock()
	c.local.Close()
	for _, conn := range c.remotes {
		conn.Close()
	}
	c.remotesLock.Unlock()
}
