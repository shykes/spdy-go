
package main

import (
    "os"
    "io"
    "github.com/shykes/spdy.go"
    "net/http"
    "log"
)

func main() {
    var err error
    session, err := spdy.DialTCP(":4242", nil)
    if err != nil {
        log.Fatal(err)
    }
    stream, err := session.OpenStream(&http.Header{})
    if err != nil {
        log.Fatal(err)
    }
    written , err := io.Copy(os.Stdout, spdy.NewStreamBytesReader(stream.Input))
    if err != nil {
        log.Fatal("copy: %s (copied %d bytes)\n", err, written)
    }
    log.Printf("Done. Wrote %d bytes\n", written)
    <-make(chan bool)
}
