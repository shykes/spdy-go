package spdy

import (
	"net/http"
	"fmt"
	"log"
)

type ResponseWriter struct {
	*Stream
	headers	*http.Header
	sentHeaders bool
}

func (w *ResponseWriter) Header() http.Header {
	if w.headers == nil {
		headers := make(http.Header)
		w.headers = &headers
	}
	return *w.headers
}

func (w *ResponseWriter) Write(data []byte) (int, error) {
	if !w.sentHeaders {
		w.WriteHeader(http.StatusOK)
	}
	debug("Sending %v\n", data)
	err := w.WriteDataFrame(data, false)
	if err != nil {
		debug("error: %s", err)
		return 0, err
	}
	return len(data), nil
}


func (w *ResponseWriter) WriteHeader(status int) {
	fin := status == 0 // Status=0 will half-close the stream 
	debug("WriteHeader() header = %v\n", w.Header())
	if w.Output.Headers.Get("status") == "" {
		w.Header().Set("status", fmt.Sprintf("%d", status))
	}
	if w.Output.nFramesIn == 0 {
		if w.local {
			w.Syn(w.headers, fin)
		} else {
			w.Reply(w.headers, fin)
		}
	} else if w.headers != nil {
		debug("Sending headers frame: %v\n", w.headers)
		if err := w.WriteHeadersFrame(w.headers, fin); err != nil {
			log.Printf("Error while writing headers frame: %s\n", err)
			return
		}
	}
	w.sentHeaders = true
}
