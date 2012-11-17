
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


/* On stream creation */
func (server *Server) ServeSPDY(stream *myspdy.Stream) {
    fmt.Printf("-> %#v\n", *stream.Input.Headers())
    stream.Output.Headers().Add("foo", "bar")
    stream.Output.Headers().Add(":status", "200")
    stream.Output.SendHeaders(false)
    if stream.Id == 1 {
        sendLinesAsync(stream.Output, bufio.NewReader(os.Stdin))
    }
    dumpStream(stream)
}


func dumpStream(stream *myspdy.Stream) {
    for {
        data, headers, err := stream.Input.Receive()
        if err == io.EOF {
            return
        } else if err != nil {
            log.Fatal(err)
        }
        // FIXME: buffer frames and print one line at a time
        if data != nil {
            os.Stdout.Write([]byte(fmt.Sprintf("[%d] %s\n", stream.Id, *data)))
        } else if headers != nil {
            os.Stdout.Write([]byte(fmt.Sprintf("[%d] [HEADERS] %s\n", stream.Id, *headers)))
        }
    }
}

func sendLinesAsync(output *myspdy.StreamWriter, source *bufio.Reader) chan bool {
    sync := make(chan bool)
    go func() {
        output.SendLines(source)
        sync<-true
    }()
    return sync
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
            log.Fatal(err)
        }
        stream, err := session.OpenStream(headers)
        if err != nil {
            log.Fatal(err)
        }
        lock := sendLinesAsync(stream.Output, bufio.NewReader(os.Stdin))
        dumpStream(stream)
        <-lock
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

