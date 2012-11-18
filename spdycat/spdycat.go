
package main

import (
    "io"
    "bufio"
    "os"
    "github.com/shykes/myspdy"
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
func (server *Server) ServeSPDY(stream *myspdy.Stream) {
    os.Stderr.Write([]byte(fmt.Sprintf("[%d] %s\n", stream.Id, headersString(*stream.Input.Headers()))))
    stream.Output.Headers().Add(":status", "200")
    stream.Output.SendHeaders(false)
    if stream.Id == 1 {
        processStream(stream)
        os.Exit(0)
    } else {
        <-dumpStreamAsync(stream)
    }
}


func dumpStreamAsync(stream *myspdy.Stream) chan bool {
    lock := make(chan bool)
    go func () {
        for {
            data, headers, err := stream.Input.Receive()
            if err != nil && err != io.EOF {
                log.Fatal("Error dumping stream: %s", err)
            }
            // FIXME: buffer frames and print one line at a time
            if data != nil && len(*data) != 0 {
                os.Stdout.Write([]byte(fmt.Sprintf("[%d] %s\n", stream.Id, *data)))
            } else if headers != nil {
                os.Stderr.Write([]byte(fmt.Sprintf("[%s] %s\n", stream.Id, headersString(*stream.Input.Headers()))))
            }
            if err == io.EOF {
                lock<-true
                return
            }
        }
    }()
    return lock
}


func sendLinesAsync(output *myspdy.StreamWriter, source *bufio.Reader) chan bool {
    sync := make(chan bool)
    go func() {
        output.SendLines(source)
        sync<-true
    }()
    return sync
}


func processStream(stream *myspdy.Stream) {
    stdin_lock := sendLinesAsync(stream.Output, bufio.NewReader(os.Stdin))
    stream_lock := dumpStreamAsync(stream)
    select {
        case _ = <-stdin_lock: {
            // stdin closed. Waiting for stream to close
            err := stream.Output.Close()
            if err != nil {
                log.Fatal("Error closing stream: %s", err)
            }
            <-stream_lock
            // stream has closed
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
        myspdy.ServeTCP(addr, server)
    } else {
        session, err := myspdy.DialTCP(addr, server)
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

