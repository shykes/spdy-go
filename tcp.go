
package spdy

import (
    "net"
    "log"
)

/* Listen on a TCP port, and pass new connections to a handler */
func ServeTCP(addr string, handler Handler) {
    debug("Listening to %s\n", addr)
    listener, err := net.Listen("tcp", addr)
    if err != nil {
        log.Fatal(err)
    }
    for {
        conn, err := listener.Accept()
        if err != nil {
            log.Fatal(err)
        }
        session, err := NewSession(conn, handler, true)
        if err != nil {
            log.Fatal(err)
        }
        go session.Run()
    }
}


/* Connect to a remote tcp server and return an RPCClient object */

func DialTCP(addr string, handler Handler) (*Session, error) {
    debug("Connecting to %s\n", addr)
    conn, err := net.Dial("tcp", addr)
    if err != nil {
        log.Fatal(err)
    }
    session, err := NewSession(conn, handler, false)
    if err != nil {
        return nil, err
    }
    go session.Run()
    return session, nil
}
