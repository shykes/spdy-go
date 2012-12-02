
package spdy

import (
    "crypto/tls"
    "net"
)

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
