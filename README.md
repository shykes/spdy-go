# spdy.go: a SPDY implementation for humans, written in Golang


## About

spdy.go is a SPDY implementation for humans, written in Golang. It is alpha software and should not be used in production.


Author: Solomon Hykes <solomon@dotcloud.com>
URL: http://github.com/shykes/spdy.go


## Example

    $ ( echo "Hi from server" | go run spdycat.go -l :4242 & echo "Hi from client" | go run spdycat.go :4242 & wait; )
