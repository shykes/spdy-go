package main

import (
	"flag"
	"fmt"
	"github.com/shykes/spdy-go"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

type Server struct{}

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

func (server *Server) ServeSPDY(stream *spdy.Stream) {
	stream.Output.Headers().Add(":status", "200")
	stream.Output.SendHeaders(false)
	processStream(stream)
}

func processStream(stream *spdy.Stream) {
	if stream.Id == 1 {
		go func() {
			_, err := io.Copy(stream.Output, os.Stdin)
			if err != nil {
				fmt.Printf("Error while sending to stream: %v\n", err)
				stream.Input.Error(err)
				stream.Output.Error(err)
			} else {
				os.Exit(0)
			}
		}()
	}
	_, err := io.Copy(os.Stdout, stream.Input)
	if err != nil {
		fmt.Printf("Error while printing stream: %v\n", err)
		stream.Input.Error(err)
		stream.Output.Error(err)
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
