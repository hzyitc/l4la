package l4la

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hzyitc/go-notify"
	"github.com/hzyitc/l4la/buffer"
	"github.com/hzyitc/mnh/log"
)

type Conn struct {
	ctx    context.Context
	cancel context.CancelFunc

	remotesLock   sync.Mutex
	remotes       []net.Conn
	remotesNotify notify.Notify

	readDeadline time.Time
	read         uint64
	readBuf      *buffer.Buffer
	readNotify   notify.Notify

	writeDeadline time.Time
	selected      int
	write         uint64
}

var byteOrder = binary.BigEndian

type header struct {
	Offset uint64
	Size   uint16
}

func NewConn(ctx context.Context) (*Conn, error) {
	ctx, cancel := context.WithCancel(ctx)

	c := &Conn{
		ctx:    ctx,
		cancel: cancel,

		remotesLock:   sync.Mutex{},
		remotes:       nil,
		remotesNotify: notify.Notify{},

		readDeadline: time.Time{},
		read:         0,
		readBuf:      buffer.NewBuffer(),
		readNotify:   notify.Notify{},

		writeDeadline: time.Time{},
		selected:      0,
		write:         0,
	}

	go func() {
		<-c.ctx.Done()
		c.remotesLock.Lock()
		for _, conn := range c.remotes {
			conn.Close()
		}
		c.remotesLock.Unlock()
	}()

	return c, nil
}

func (c *Conn) WaitClose() <-chan struct{} {
	return c.ctx.Done()
}

func (c *Conn) Close() error {
	c.cancel()
	return nil
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

		c.readNotify.NotifyAll()
	}
}

func (c *Conn) Read(b []byte) (n int, err error) {
	for {
		ch := c.readNotify.Wait()

		_, buf := c.readBuf.Pop(len(b), true)
		if buf != nil {
			return copy(b, buf), nil
		}

		select {
		case <-c.ctx.Done():
			return 0, ErrClose
		case <-timeAfter(c.readDeadline):
			return 0, ErrTimeout
		case <-ch:
		}
	}
}

func (c *Conn) Write(b []byte) (n int, err error) {
	for {
		ch := c.remotesNotify.Wait()

		c.remotesLock.Lock()
		if len(c.remotes) >= 2 {
			break
		}
		c.remotesLock.Unlock()

		select {
		case <-c.ctx.Done():
			return 0, ErrClose
		case <-timeAfter(c.writeDeadline):
			return 0, ErrTimeout
		case <-ch:
		}
	}
	defer c.remotesLock.Unlock()

	conn := c.remotes[c.selected]
	c.selected = (c.selected + 1) % len(c.remotes)

	err = binary.Write(conn, byteOrder, header{
		Offset: c.write,
		Size:   uint16(len(b)),
	})
	if err != nil {
		c.Close()
		return 0, err
	}

	n, err = conn.Write(b)
	if err != nil {
		c.Close()
		return 0, err
	}
	if n != len(b) {
		c.Close()
		return 0, err
	}

	c.write += uint64(len(b))

	return n, nil
}

func (c *Conn) LocalAddr() net.Addr {
	return &Addr{}
}

func (c *Conn) RemoteAddr() net.Addr {
	return &Addr{}
}

func (c *Conn) SetDeadline(t time.Time) error {
	err := c.SetReadDeadline(t)
	if err == nil {
		return err
	}

	return c.SetWriteDeadline(t)
}

func (c *Conn) SetReadDeadline(t time.Time) error {
	c.readDeadline = t
	return nil
}

func (c *Conn) SetWriteDeadline(t time.Time) error {
	c.writeDeadline = t
	return nil
}
