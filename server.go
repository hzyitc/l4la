package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"

	"github.com/google/uuid"
	"github.com/hzyitc/mnh/log"
)

type Server struct {
	service string

	conns sync.Map
}

func NewServer(port int, server string) {
	local := "0.0.0.0:" + strconv.Itoa(port)
	listener, err := net.Listen("tcp", local)
	if err != nil {
		log.Error(err.Error())
		return
	}
	log.Info("Listening at " + listener.Addr().String())

	s := &Server{
		server,

		sync.Map{},
	}
	s.main(listener)
}

func (s *Server) main(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Error("server_main error", err.Error())
			return
		}

		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	log.Info("New connection from " + conn.RemoteAddr().String())

	buf := make([]byte, 16)
	_, err := io.ReadFull(conn, buf)
	if err != nil {
		log.Error("server_handle read error:", err.Error())
		conn.Close()
		return
	}

	id, err := uuid.FromBytes(buf)
	if err != nil {
		log.Error("server_handle read uuid error:", err.Error())
		conn.Close()
		return
	}

	if id == uuid.Nil {
		service, err := net.Dial("tcp", s.service)
		if err != nil {
			log.Error("server_handle dial error:", err.Error())
			conn.Close()
			return
		}

		id = uuid.New()

		c, err := NewConn(context.TODO(), service)
		if err != nil {
			log.Error("server_handle newRemoteConn error:", err.Error())
			conn.Close()
			return
		}
		log.Info("Created new connection", id.String())
		s.conns.Store(id, c)

		buf, err := id.MarshalBinary()
		if err != nil {
			log.Error("server_handle id.MarshalBinary error:", err.Error())
			conn.Close()
			return
		}

		n, err := conn.Write(buf)
		if err != nil {
			log.Error("server_handle write error:", err.Error())
			conn.Close()
			return
		}
		if n != len(buf) {
			log.Error("server_handle write error:", fmt.Errorf("sent %d bytes instand of %d bytes", n, len(buf)))
			conn.Close()
			return
		}

		log.Info("New connection to ", id.String())
		c.addRemoteConn(conn)
	} else {
		v, ok := s.conns.Load(id)
		if !ok {
			log.Error("Unknown conn id: ", id.String())
			conn.Close()
			return
		}

		c := v.(*Conn)

		log.Info("New connection to ", id.String())
		c.addRemoteConn(conn)
	}

}
