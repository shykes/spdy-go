
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
)

/* Replace this with anything which implements http.Handler */
var webapp http.Handler = http.FileServer(http.Dir("."))

func main() {
    flag.Parse()
    spdy.ServeTCP(flag.Args()[0], &spdy.HTTPHandler{webapp})
}
