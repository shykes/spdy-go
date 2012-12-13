package wire

import (
	"log"
	"net"
)

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
		log.Fatal(err)
	}
	return Serve(conn, handler, false)
}
