
package myspdy

import (
    "net/http"
)

/*
** Run `f` in a new goroutine and return a channel which will receive
** its return value
*/

func promise(f func() error) chan error {
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
