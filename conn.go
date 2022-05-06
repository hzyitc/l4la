package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/hzyitc/mnh/log"
)

type Conn struct {
	local   net.Conn
	remotes []net.Conn

	ctx    context.Context
	cancel context.CancelFunc

	cond sync.Cond

	read uint64

	selected int
	write    uint64
}

func NewConn(ctx context.Context, local net.Conn) (*Conn, error) {
	ctx, cancel := context.WithCancel(ctx)

	c := &Conn{
		local:   local,
		remotes: nil,

		ctx:    ctx,
		cancel: cancel,

		cond: *sync.NewCond(&sync.Mutex{}),

		read: 0,

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
	c.cond.L.Lock()
	c.remotes = append(c.remotes, conn)
	c.cond.L.Unlock()

	go func() {
		c.handleRemote(c.ctx, conn)
		c.cancel()
	}()

	c.cond.Broadcast()
}

func (c *Conn) handleRemote(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	intbuf := make([]byte, 16)
	buf := make([]byte, 16+9000)
	for {
		_, err := io.ReadFull(conn, intbuf)
		if err != nil {
			log.Error("handleRemote read error:", err.Error())
			return
		}

		offset := binary.BigEndian.Uint64(intbuf[8*0:])
		size := int(binary.BigEndian.Uint64(intbuf[8*1:]))

		_, err = io.ReadFull(conn, buf[:size])
		if err != nil {
			log.Error("handleRemote read error:", err.Error())
			return
		}

		c.cond.L.Lock()
		for c.read != offset {
			select {
			case <-ctx.Done():
				c.cond.L.Unlock()
				return
			default:
			}

			c.cond.Wait()
		}

		n, err := c.local.Write(buf[:size])
		if err != nil {
			log.Error("handleRemote write error:", err.Error())
			c.cond.L.Unlock()
			return
		}
		if n != size {
			log.Error("handleRemote write error:", fmt.Sprintf("sent %d bytes instand of %d bytes", n, size))
			c.cond.L.Unlock()
			return
		}

		c.read += uint64(size)
		c.cond.L.Unlock()
		c.cond.Broadcast()
	}
}

func (c *Conn) handleLocal(ctx context.Context) {
	defer c.local.Close()

	buf := make([]byte, 16+9000)
	for {
		size, err := c.local.Read(buf[16:])
		if err != nil {
			log.Error("handleLocal error:", err.Error())
			return
		}

		conn := c.remotes[c.selected]
		c.selected = (c.selected + 1) % len(c.remotes)

		binary.BigEndian.PutUint64(buf[8*0:], c.write)
		binary.BigEndian.PutUint64(buf[8*1:], uint64(size))

		n, err := conn.Write(buf[:16+size])
		if err != nil {
			log.Error("handleLocal write error:", err.Error())
			return
		}
		if n != len(buf[:16+size]) {
			log.Error("handleLocal write error:", fmt.Errorf("sent %d bytes instand of %d bytes", n, len(buf[:16+size])))
		}

		c.write += uint64(size)
	}
}

func (c *Conn) main() {
	c.cond.L.Lock()
	for len(c.remotes) < 2 {
		select {
		case <-c.ctx.Done():
		default:
		}
		c.cond.Wait()
	}
	c.cond.L.Unlock()

	go func() {
		c.handleLocal(c.ctx)
		c.cancel()
	}()

	<-c.ctx.Done()
	for _, conn := range c.remotes {
		conn.Close()
	}
}
