
/*
** webapp.go ADDR: serve a standard http handler over spdy
**
** Usage:
**
**      go run webapp.go :4242
**
**      spdycat :4242 :path=/
*/

package main

import (
    "net/http"
    "github.com/shykes/spdy-go"
    "flag"
    "log"
)

/* Replace this with anything which implements http.Handler */
var webapp http.Handler = http.FileServer(http.Dir("."))

func main() {
    tls := flag.Bool("t", false, "Enable TLS")
    flag.Parse()
    var err error
    if *tls {
	    err = spdy.ListenAndServeTLS(flag.Args()[0], "cert.pem", "key.pem", spdy.WrapHTTPHandler(webapp))
    } else {
	    err = spdy.ListenAndServeTCP(flag.Args()[0], spdy.WrapHTTPHandler(webapp))
    }
    if err != nil {
	log.Fatal(err)
    }
}
