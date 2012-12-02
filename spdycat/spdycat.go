
package main

import (
    "io"
    "bufio"
    "os"
    "github.com/shykes/spdy.go"
    "flag"
    "log"
    "fmt"
    "strings"
    "net/http"
)


type Server struct {}


func headersString(headers http.Header) string {
    var s string
    for key := range headers {
        if len(s) != 0 {
            s = s + " "
        }
        s += fmt.Sprintf("%s=%s", key, headers.Get(key))
    }
    return s
}

/* On stream creation */
func (server *Server) ServeSPDY(stream *spdy.Stream) {
    stream.Output.Headers().Add(":status", "200")
    stream.Output.SendHeaders(false)
    if stream.Id == 1 {
        processStream(stream)
        os.Exit(0)
    } else {
        <-dumpStreamAsync(stream)
    }
}


func dumpStreamAsync(stream *spdy.Stream) chan bool {
    lock := make(chan bool)
    go func () {
        io.Copy(os.Stdout, stream.Input)
        lock<-true
    }()
    return lock
}


func sendLinesAsync(output *spdy.StreamWriter, source *bufio.Reader) chan bool {
    sync := make(chan bool)
    go func() {
        io.Copy(output, source)
        sync<-true
    }()
    return sync
}


func processStream(stream *spdy.Stream) {
    stdin_lock := sendLinesAsync(stream.Output, bufio.NewReader(os.Stdin))
    stream_lock := dumpStreamAsync(stream)
    select {
        case _ = <-stdin_lock: {
            // stdin closed. Waiting for stream to close
            err := stream.Output.Close()
            if err != nil {
                log.Fatal("Error closing stream: %s", err)
            }
        }
        case _ = <-stream_lock: {
            // stream input closed. Closing output
            err := stream.Output.Close()
            if err != nil {
                log.Fatal("Error closing stream: %s", err)
            }
        }
    }
    err := stream.Session().Close()
    if err != nil {
        log.Fatal("Error closing session: %s", err)
    }
}


func main() {
    listen := flag.Bool("l", false, "Listen to <addr>")
    flag.Parse()
    addr := flag.Args()[0]
    headers := extractHeaders(flag.Args()[1:])
    server := &Server{} // FIXME: find another name for Server since it is used by both sides
    if *listen {
        err := spdy.ListenAndServeTCP(addr, server)
        /*
         * // Uncomment to serve over TLS instad of raw TCP
         * err := spdy.ListenAndServeTLS(addr, "cert.pem", "key.pem", server)
         */
        if err != nil {
            log.Fatal("Listen: %s", err)
        }
    } else {
        session, err := spdy.DialTCP(addr, server)
        if err != nil {
            log.Fatal("Error connecting: %s", err)
        }
        stream, err := session.OpenStream(headers)
        if err != nil {
            log.Fatal("Error opening stream: %s", err)
        }
        processStream(stream)
    }
}

func extractHeaders(args []string) *http.Header {
    headers := http.Header{}
    for _, keyvalue := range args {
        pair := strings.SplitN(keyvalue, "=", 2)
        headers.Set(pair[0], pair[1])
    }
    return &headers
}

