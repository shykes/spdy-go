

package myspdy

import (
    "code.google.com/p/go.net/spdy"
    "io"
    "log"
    "bufio"
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

func DialTCP(addr string) (*Session, error) {
    debug("Connecting to %s\n", addr)
    conn, err := net.Dial("tcp", addr)
    if err != nil {
        log.Fatal(err)
    }
    return NewSession(conn, conn, nil, false)
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


func (session *Session) OpenStream(headers *http.Header, handler Handler) (*Stream, error) {
    newId := session.nextId()
    stream := newStream(session, newId, handler)
    session.lastStreamId = newId
    session.streams[newId] = stream
    err := stream.SendSyn(headers)
    if err != nil {
        return nil, err
    }
    return stream, nil
}


/*
** Listen for new frames and process them. Inbound streams will be passed to `onRequest`.
*/

func (session *Session) Run() {
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
            session.onDataFrame(dframe)
        /* Did we receive a syn_stream control frame? */
        } else if synframe, ok := frame.(*spdy.SynStreamFrame); ok {
            session.onSynStreamFrame(synframe)
        /* Did we receive a syn_reply control frame */
        } else if synReplyFrame, ok := frame.(*spdy.SynReplyFrame); ok {
            session.onSynReplyFrame(synReplyFrame)
        }
    }
}

/* Process a new DATA frame */
func (session *Session) onDataFrame(dframe *spdy.DataFrame) {
    debug("Received data frame: %s\n", dframe)
    stream, ok := session.streams[dframe.StreamId]
    if ok {
        stream.onDataFrame(dframe)
    } else {
        debug("Warning: received data for non-open stream. Dropping\n")
    }
}

/* Process a new SYN_STREAM frame */
func (session *Session) onSynStreamFrame(synframe *spdy.SynStreamFrame) {
    debug("Received syn_stream frame for stream id=%d headers=%s\n", synframe.StreamId, synframe.Headers)
    _, ok := session.streams[synframe.StreamId]
    if !ok {
        /* FIXME check for valid ID */
        debug("Opening new stream: %s\n", synframe.StreamId)
        stream := newStream(session, synframe.StreamId, session.handler)
        session.streams[synframe.StreamId] = stream
        stream.onSynStreamFrame(synframe)
        /* Send SYN_REPLY with basic headers before sending any data */
    } else {
        debug("Warning: peer trying to open already open stream. Dropping\n")
    }
}

/* Process a new SYN_REPLY frame */
func (session *Session) onSynReplyFrame(synReplyFrame *spdy.SynReplyFrame) {
    id := synReplyFrame.StreamId
    debug("Received syn_reply frame for stream id=%d\n", id)
    if !session.isLocalId(id) {
        debug("Warning: received reply for stream id %d which we can't legally create. Dropping\n", id)
        return
    }
    stream, ok := session.streams[id]
    if !ok {
        debug("Warning: received reply for unknown stream id=%d. Dropping\n", id)
        return
    }
    stream.onSynReplyFrame(synReplyFrame)
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


/*
** Stream-specific operations
** (FIXME: wrap in a Stream type?)
**
*/

type Stream struct {
    session         *Session
    Id              uint32
    InHeaders       http.Header
    OutHeaders      http.Header
    InboundData     chan *[]byte
    handler         Handler
}


func newStream(session *Session, id uint32, handler Handler) *Stream {
    return &Stream{
        session,
        id,
        http.Header{},
        http.Header{},
        make(chan *[]byte, STREAM_BUFFER_SIZE),
        handler,
    }
}


func (stream *Stream) SendData(data []byte) error {
    debug("Sending data: %s\n", data)
    return stream.session.WriteFrame(&spdy.DataFrame{
        StreamId:   stream.Id,
        Data:       data,
    })
}


func (stream *Stream) SendLines(lines *bufio.Reader) error {
    for {
        line, _, err := lines.ReadLine()
        if err == io.EOF {
            debug("Received EOF from input\n")
            return nil
        } else if err != nil {
            return err
        }
        if stream.SendData(line) != nil {
            return err
        }
   }
   return nil
}

/* Block until `stream` is closed  (FIXME: not implemented) */
func (stream *Stream) Wait() {
    <-make(chan bool) /* Hang forever */
}


func (stream *Stream) onDataFrame(frame *spdy.DataFrame) {
    debug("[STREAM %d] Received data frame %s\n", stream.Id, frame.Data)
    stream.InboundData <- &frame.Data
}

func (stream *Stream) onSynStreamFrame(frame *spdy.SynStreamFrame) {
    updateHeaders(&stream.InHeaders, &frame.Headers) /* We received new headers */
    go stream.handler.ServeSPDY(stream)
}

func (stream *Stream) onSynReplyFrame(frame *spdy.SynReplyFrame) {
    updateHeaders(&stream.InHeaders, &frame.Headers) /* We received new headers */
    go stream.handler.ServeSPDY(stream)
}

func (stream *Stream) SendSyn(headers *http.Header) error {
    updateHeaders(&stream.OutHeaders, headers)
    return stream.session.WriteFrame(&spdy.SynStreamFrame{
            StreamId: stream.Id,
            CFHeader: spdy.ControlFrameHeader{},
            Headers: *headers,
    })
}


func (stream *Stream) SendSynReply(headers *http.Header, final bool) error {
    updateHeaders(&stream.OutHeaders, headers)
    var flags spdy.ControlFlags
    if final {
        debug("Setting FLAG_FIN to close stream %d\n", stream.Id)
        flags |= spdy.ControlFlagFin
    }
    return stream.session.WriteFrame(&spdy.SynReplyFrame{
        CFHeader:       spdy.ControlFrameHeader{Flags: flags},
        Headers:        *headers,
        StreamId:       stream.Id,
    })
}

/*
** Call the stream's handler.
**
*/
func (stream *Stream) Run() {
    go stream.handler.ServeSPDY(stream)
}


func updateHeaders(headers *http.Header, newHeaders *http.Header) {
    for key, values := range *newHeaders {
        for _, value := range values {
            headers.Add(key, value)
        }
    }
}
