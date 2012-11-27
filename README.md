# spdy.go: a SPDY Golang implementation for humans


## About

spdy.go is a SPDY implementation for humans, written in Golang. It offers a higher-level API to the buit-in package available at http://code.google.com/p/go.net/spdy


## Status

spdy.go is under active development. It is alpha software and should not be used in production. Contributions and patches are welcome!

Author: Solomon Hykes <solomon@dotcloud.com>
URL: http://github.com/shykes/spdy.go


## Example

    $ ( echo "Hi from server" | go run spdycat.go -l :4242 & echo "Hi from client" | go run spdycat.go :4242 & wait; )
