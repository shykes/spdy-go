
package spdy

import (
    "log"
    "net/http"
    "net/url"
    "fmt"
)

/*
** HTTPHandler: wrap any HTTP handler and serve it over SPDY
**
** Example:
**
**  import ("net/http"; "github.com/shykes/myspdy")
**  [...]
**  myspdy.ServeTCP(":8080", &myspdy.HTTPHandler{http.FileServer(http.Dir("/tmp"))})
**
*/

func WrapHTTPHandler(handler http.Handler) Handler {
    h := HandlerFunc(func(stream *Stream) {
        request, err := http.NewRequest(
            stream.Input.Headers().Get(":method"),
            (&url.URL{
                Host:   stream.Input.Headers().Get("host"),
                Path:   stream.Input.Headers().Get("url"),
            }).String(),
            stream.Input)
        if err != nil {
            log.Fatal(err)
        }
        handler.ServeHTTP(&ResponseWriter{StreamWriter:*stream.Output}, request)
    })
    return &h
}


/*
** ResponseWriter
*/

type ResponseWriter struct {
    StreamWriter
    nFrames int
}

func (writer *ResponseWriter) Header() http.Header {
    return *writer.Headers()
}

func (writer *ResponseWriter) Write(data []byte) (int, error) {
	if writer.nFrames == 0 {
		writer.WriteHeader(200)
	}
	writer.nFrames += 1
	return writer.StreamWriter.Write(data)
}

func (writer *ResponseWriter) WriteHeader(status int) {
    writer.Headers().Set("status", fmt.Sprintf("%d", status))
    writer.Headers().Set("version", "HTTP/1.1")
    writer.SendHeaders(false)
    writer.nFrames += 1
}
