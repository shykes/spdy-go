
package main

import (
	"github.com/shykes/spdy-go/wire"
	"net/http"
	"code.google.com/p/go.net/spdy"
	"log"
	"fmt"
	"bytes"
	"text/template"
)

func main() {
	tpl, err := template.New("response").Parse("<html><table><tr><th>KEY</th><th>VALUE</th></tr>{{range $key, $value := .}}<tr><td>{{$key}}</td><td>{{$value}}</td></tr>{{end}}</html>")
	if err != nil {
		log.Fatal(err)
	}
	handler := wire.HandlerFunc(func(s *wire.Stream) {
		frame, err := s.Input.ReadFrame()
		if err != nil {
			log.Fatal(err)
		}
		response := &bytes.Buffer{}
		tpl.Execute(response, wire.FrameHeaders(frame))
		headers := http.Header{}
		headers.Add("status", "200")
		headers.Add("version", "HTTP/1.1")
		headers.Add("content-type", "text/html")
		s.Output.WriteFrame(&spdy.SynReplyFrame{StreamId: s.Id, Headers: headers})
		s.Output.WriteFrame(&spdy.DataFrame{StreamId:	s.Id, Data: response.Bytes(), Flags: spdy.DataFlagFin})
	})
	fmt.Printf("Listening on :4242\n")
	if err := wire.ListenAndServeTLS(":4242", "cert.pem", "key.pem", &handler); err != nil {
		log.Fatal(err)
	}
}
