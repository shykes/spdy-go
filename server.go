package spdy

import (
	"log"
	"crypto/tls"
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
	session := NewSession(handler, server)
	go session.Serve(framer)
	return session, nil
}

/* Listen on a TCP port, and pass new connections to a handler */
func ListenAndServeTCP(addr string, handler Handler) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	return ListenAndServe(listener, handler)
}

/* Connect to a remote tcp server and return a new Session */
func DialTCP(addr string, handler Handler) (*Session, error) {
	debug("Connecting to %s\n", addr)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	return Serve(conn, handler, false)
}

func ListenAndServeTLS(addr, certFile, keyFile string, handler Handler) error {
	if addr == "" {
		addr = ":https"
	}
	config := &tls.Config{}
	config.NextProtos = []string{"spdy/2"}

	var err error
	config.Certificates = make([]tls.Certificate, 1)
	config.Certificates[0], err = tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return err
	}

	conn, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	tlsListener := tls.NewListener(conn, config)
	return ListenAndServe(tlsListener, handler)
}

func DialTLS(addr string, handler Handler) (*Session, error) {
	config := &tls.Config{}
	config.NextProtos = []string{"spdy/2"}
	config.InsecureSkipVerify = true //FIXME: load a root CA instead
	conn, err := tls.Dial("tcp", addr, config)
	if err != nil {
		return nil, err
	}
	return Serve(conn, handler, false)
}
