
package main

import (
    "io"
    "bufio"
    "os"
    "github.com/shykes/myspdy"
    "flag"
    "log"
    "fmt"
    "net/http"
)


type Server struct {}


/* On stream creation */
func (server *Server) ServeSPDY(stream *myspdy.Stream) {
    fmt.Printf("-> %s\n", *stream.Input.Headers())
    stream.Output.Headers().Add("foo", "bar")
    stream.Output.Headers().Add(":status", "200")
    stream.Output.SendHeaders(false)
    if stream.Id == 1 || stream.Id == 2 {
        go stream.Output.SendLines(bufio.NewReader(os.Stdin))
    }
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
            os.Stdout.Write([]byte(fmt.Sprintf("[%d] [HEADERS] %s\n", stream.Id, headers)))
        }
    }
}


func main() {
    listen := flag.Bool("l", false, "Listen to <addr>")
    flag.Parse()
    addr := flag.Args()[0]
    server := &Server{} // FIXME: find another name for Server since it is used by both sides
    if *listen {
        myspdy.ServeTCP(addr, server)
    } else {
        session, err := myspdy.DialTCP(addr, server)
        if err != nil {
            log.Fatal(err)
        }
        _, err = session.OpenStream(&http.Header{}, server)
        if err != nil {
            log.Fatal(err)
        }
    }
    <-make(chan bool)
}
