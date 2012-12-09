package spdy

import (
	"log"
	"net/http"
	"os"
)

/*
** Run `f` in a new goroutine and return a channel which will receive
** its return value
 */

func Promise(f func() error) chan error {
	ch := make(chan error)
	go func() {
		ch <- f()
	}()
	return ch
}

/*
** Add the contents of `newHeaders` to `headers`
 */

func updateHeaders(headers *http.Header, newHeaders *http.Header) {
	for key, values := range *newHeaders {
		for _, value := range values {
			headers.Add(key, value)
		}
	}
}

/*
** Output a message only if the DEBUG env variable is set
 */

var DEBUG bool = false

func debug(msg string, args ...interface{}) {
	if DEBUG || (os.Getenv("DEBUG") != "") {
		log.Printf(msg, args...)
	}
}

type HandlerFunc func(*Stream)

func (f *HandlerFunc) ServeSPDY(s *Stream) {
	(*f)(s)
}
