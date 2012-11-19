
package myspdy

import (
    "log"
    "net/http"
    "net/url"
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

type HTTPHandler struct {
    http.Handler
}


func (handler *HTTPHandler) ServeSPDY(stream *Stream) {
    request, err := http.NewRequest(
        stream.Input.Headers().Get(":method"),
        (&url.URL{
            Host:   stream.Input.Headers().Get("host"),
            Path:   stream.Input.Headers().Get(":path"),
        }).String(),
        &Body{*stream.Input})
    if err != nil {
        log.Fatal(err)
    }
    handler.ServeHTTP(&ResponseWriter{*stream.Output}, request)
}



/*
** Body
*/


type Body struct {
    StreamReader
}

func (body *Body) Read(dest []byte) (int, error) {
    for {
        data, _, err := body.Receive()
        if err != nil {
            return 0, err
        }
        if data == nil && len(*data) != 0 {
            copy(dest, *data)
            return len(*data), nil
        }
    }
    return 0, nil
}



/*
** ResponseWriter
*/

type ResponseWriter struct {
    StreamWriter
}

func (writer *ResponseWriter) Header() http.Header {
    return *writer.Headers()
}


func (writer *ResponseWriter) WriteHeader(status int) {
    writer.Headers().Add(":status", string(status))
    writer.SendHeaders(false)
}


func (writer *ResponseWriter) Write(data []byte) (int, error) {
    err := writer.Send(&data)
    if err != nil {
        return 0, err
    }
    return len(data), nil
}
