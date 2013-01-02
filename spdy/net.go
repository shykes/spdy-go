package spdy

import (
	"net"
)

func ListenAndServe(listener net.Listener, handler Handler) error {
	debug("Listening to %s\n", listener.Addr())
	for {
		conn, err := listener.Accept()
		debug("New connection from %s\n", conn.RemoteAddr())
		if err != nil {
			return err
		}
		if _, err := Serve(conn, handler, true); err != nil {
			return err
		}
	}
	return nil
}


func Serve(conn net.Conn, handler Handler, server bool) (*Session, error) {
	framer, err := NewFramer(conn, conn)
	if err != nil {
		return nil, err
	}
	return NewSession(framer, handler, server), nil
}
