

package myspdy

import (
    "code.google.com/p/go.net/spdy"
    "io"
    "log"
    "os"
    "net"
    "net/http"
)

var DEBUG bool = false
var STREAM_BUFFER_SIZE = 1000

func debug(msg string, args... interface{}) {
    if DEBUG || (os.Getenv("DEBUG") != "") {
        log.Printf(msg, args...)
    }
}


type Handler interface {
    ServeSPDY(stream *Stream)
}


/*
** type Session
**
** A high-level representation of a SPDY connection
**  <<
**      connection: A transport-level connection between two endpoints.
**      session: A synonym for a connection.
**  >>
**  (http://tools.ietf.org/html/draft-mbelshe-httpbis-spdy-00#section-1.2)
** 
*/

type Session struct {
    *spdy.Framer
    Server              bool                    // Are we the server? (necessary for stream ID numbering)
    lastStreamId        uint32                  // Last (and highest-numbered) stream ID we allocated
    streams             map[uint32] *Stream
    handler             Handler
}


/* Create a new Session object */
func NewSession(writer io.Writer, reader io.Reader, handler Handler, server bool) (*Session, error) {
    framer, err := spdy.NewFramer(writer, reader)
    if err != nil {
        return nil, err
    }
    return &Session{
        framer,
        server,
        0,
        make(map[uint32]*Stream),
        handler,
    }, nil
}


/* Listen on a TCP port, and pass new connections to a handler */
func ServeTCP(addr string, handler Handler) {
    debug("Listening to %s\n", addr)
    listener, err := net.Listen("tcp", addr)
    if err != nil {
        log.Fatal(err)
    }
    for {
        conn, err := listener.Accept()
        if err != nil {
            log.Fatal(err)
        }
        session, err := NewSession(conn, conn, handler, true)
        if err != nil {
            log.Fatal(err)
        }
        go session.Run()
    }
}


/* Connect to a remote tcp server and return an RPCClient object */

func DialTCP(addr string, handler Handler) (*Session, error) {
    debug("Connecting to %s\n", addr)
    conn, err := net.Dial("tcp", addr)
    if err != nil {
        log.Fatal(err)
    }
    session, err := NewSession(conn, conn, handler, false)
    if err != nil {
        return nil, err
    }
    go session.Run()
    return session, nil
}

/*
** Compute the ID which should be used to open the next stream 
** 
** Per http://tools.ietf.org/html/draft-mbelshe-httpbis-spdy-00#section-2.3.2
** <<
** If the server is initiating the stream,
**    the Stream-ID must be even.  If the client is initiating the stream,
**    the Stream-ID must be odd. 0 is not a valid Stream-ID.  Stream-IDs
**    from each side of the connection must increase monotonically as new
**    streams are created.  E.g.  Stream 2 may be created after stream 3,
**    but stream 7 must not be created after stream 9.  Stream IDs do not
**    wrap: when a client or server cannot create a new stream id without
**    exceeding a 31 bit value, it MUST NOT create a new stream.
** >>
*/
 func (session *Session) nextId() uint32 {
    if session.lastStreamId == 0 {
        if session.Server {
            return 2
        } else {
            return 1
        }
    }
    return session.lastStreamId + 2
}


func (session *Session) OpenStream(headers *http.Header) (*Stream, error) {
    newId := session.nextId()
    stream := newStream(session, newId, true)
    session.lastStreamId = newId
    session.streams[newId] = stream
    updateHeaders(stream.Output.Headers(), headers)
    stream.Output.SendHeaders(false)
    return stream, nil
}


/*
** Listen for new frames and process them. Inbound streams will be passed to `onRequest`.
*/

func (session *Session) Run() {
    debug("%s Run()", session)
    for {
        frame, err := session.ReadFrame()
        var prefix string
        if err == io.EOF {
            debug("EOF from peer\n")
            return
        } else if err != nil {
            log.Fatal(err)
        }
        debug("Received frame %s\n", prefix, frame)
        /* Did we receive a data frame? */
        if dframe, ok := frame.(*spdy.DataFrame); ok {
            stream, exists := session.streams[dframe.StreamId]
            if !exists {
                // Protocol error
                debug("Received a data frame for unknown stream id %s. Dropping.\n", dframe.StreamId)
                continue
            }
            stream.Input.Push(&dframe.Data, nil)
        /* FIXME: Did we receive a headers control frame? */
        } else if headersframe, ok := frame.(*spdy.HeadersFrame); ok {
            stream, exists := session.streams[headersframe.StreamId]
            if !exists { // Protocol error
                debug("Received headers for unknown stream id %s. Dropping.\n", headersframe.StreamId)
                continue
            }
            stream.Input.Push(nil, &headersframe.Headers)
            if headersframe.CFHeader.Flags & spdy.ControlFlagFin != 0 {
                stream.Input.Close()
            }

        /* Did we receive a syn_stream control frame? */
        } else if synframe, ok := frame.(*spdy.SynStreamFrame); ok {
            _, exists := session.streams[synframe.StreamId]
            if exists { // Protocol error
                debug("Received syn_stream frame for stream id=%s. Dropping\n", synframe.StreamId)
                continue
            }
            /* Create a new stream */
            stream := newStream(session, synframe.StreamId, false)
            session.streams[synframe.StreamId] = stream
            /* Set the initial headers */
            updateHeaders(stream.Input.Headers(), &synframe.Headers)
            /* Run the handler */
            go session.handler.ServeSPDY(stream)
        /* Did we receive a syn_reply control frame */
        } else if synReplyFrame, ok := frame.(*spdy.SynReplyFrame); ok {
            id := synReplyFrame.StreamId
            if !session.isLocalId(id) {
                debug("Warning: received reply for stream id %d which we can't legally create. Dropping\n", id)
                continue
            }
            stream, exists := session.streams[id]
            if !exists {
                debug("Warning: received reply for unknown stream id=%d. Dropping\n", id)
                continue
            }
            /* Set the initial headers */
            updateHeaders(stream.Input.Headers(), &synReplyFrame.Headers)
            stream.Input.Push(nil, &synReplyFrame.Headers)
        }
    }
}


/*
 * Return true if it's legal for `id` to be locally created
 * (eg. even-numbered if we're the server, odd-numbered if we're the client)
 */
func (session *Session) isLocalId(id uint32) bool {
    if session.Server {
        return (id % 2 == 0) /* Return true if id is even */
    }
    return (id % 2 != 0) /* Return true if id is odd */
}



func updateHeaders(headers *http.Header, newHeaders *http.Header) {
    for key, values := range *newHeaders {
        for _, value := range values {
            headers.Add(key, value)
        }
    }
}
