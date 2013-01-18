# spdy.go: a SPDY Golang implementation for humans


## About

spdy.go is a SPDY implementation for humans, written in Golang. It extends the official package [1] with a higher-level API.

[1] http://code.google.com/p/go.net/spdy


## Status

This package is under active development. It is alpha software and should not be used in production. Contributions and patches are welcome!

Author: Solomon Hykes <solomon@dotcloud.com>
URL: http://github.com/shykes/spdy-go


## Installation

0. Install GO on your computer (http://golang.org/doc/install)

1. Setup your GO environment:

    $ mkdir ~/go
    
    $ export GOPATH=~/go
    
    $ export PATH=$PATH:$GOPATH/bin

2. Install the library

    $ go get github.com/shykes/spdy-go


## Examples


### Serve a web application over spdy

    [Shell A]   $ go run examples/webapp.go -t :8080

    [Chrome]    https://localhost:8080


## Bugs & missing features

* No support for SETTINGS

* No support for GOAWAY
