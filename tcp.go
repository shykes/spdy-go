
package spdy

import (
    "net"
    "log"
)


/* Listen on a TCP port, and pass new connections to a handler */
func ServeTCP(addr string, handler Handler) error {
    listener, err := net.Listen("tcp", addr)
    if err != nil {
        log.Fatal(err)
    }
    return Serve(listener, handler)
}


func Serve(listener net.Listener, handler Handler) error {
    debug("Listening to %s\n", listener.Addr())
    for {
        conn, err := listener.Accept()
        debug("New connection from %s\n", conn.RemoteAddr())
        if err != nil {
            return err
        }
        session, err := NewSession(conn, handler, true)
        if err != nil {
            return err
        }
        go session.Run()
    }
    return nil
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
