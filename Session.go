

package myspdy

import (
    "code.google.com/p/go.net/spdy"
    "io"
    "log"
    "bufio"
    "os"
    "net/http"
)

var DEBUG bool = false

func debug(msg string, args... interface{}) {
    if DEBUG || (os.Getenv("DEBUG") != "") {
        log.Printf(msg, args...)
    }
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
    streams             map[uint32]chan *[]byte
    myStreams           map[uint32]StreamHandler         // Response handlers for all streams opened by us
}

/* A function which can be called when a new stream is created by the session's remote peer */
type StreamHandler func(session *Session, id uint32, headers *http.Header, stream <-chan *[]byte)


/* Create a new Session object */
func NewSession(writer io.Writer, reader io.Reader, server bool) (*Session, error) {
    framer, err := spdy.NewFramer(writer, reader)
    if err != nil {
        return nil, err
    }
    return &Session{
        framer,
        server,
        0,
        make(map[uint32]chan *[]byte),
        make(map[uint32]StreamHandler),
    }, nil
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


/* Initiate a new stream (this will send a SYN_STREAM frame to the peer) */
func (session *Session) OpenStream(headers http.Header, onResponse StreamHandler) (uint32) {
    newId := session.nextId()
    debug("Creating new stream with ID %d\n", newId)
    synStreamFrame := spdy.SynStreamFrame{
            StreamId: newId,
            CFHeader: spdy.ControlFrameHeader{},
            Headers: headers,
        }
    err := session.WriteFrame(&synStreamFrame)
    if err != nil {
        log.Fatal(err)
    }
    session.lastStreamId = newId
    session.myStreams[newId] = onResponse
    return newId
}

/* Send data on an open stream */
func (session *Session) send_data(streamId uint32, data []byte) {
    debug("Sending data: %s\n", data)
    err := session.WriteFrame(&spdy.DataFrame{
        StreamId:   streamId,
        Data:       data,
    })
    if err != nil {
        log.Fatal("WriteFrame: %s", err)
    }
}




/* Listen for new frames and process them */
func (session *Session) run(onRequest StreamHandler) {
    for {
        frame, err := session.ReadFrame()
        if err == io.EOF {
            debug("EOF from peer\n")
            return
        } else if err != nil {
            log.Fatal(err)
        }
        debug("Received frame: %s\n", frame)
        /* Did we receive a data frame? */
        if dframe, ok := frame.(*spdy.DataFrame); ok {
            debug("Received data frame: %s\n", dframe)
            stream, ok := session.streams[dframe.StreamId]
            if ok {
                stream <- &dframe.Data
            } else {
                debug("Warning: received data for non-open stream. Dropping\n")
            }
        /* Did we receive a syn_stream control frame? */
        } else if synframe, ok := frame.(*spdy.SynStreamFrame); ok {
            debug("Received syn_stream frame for stream id=%d headers=%s\n", synframe.StreamId, synframe.Headers)
            _, ok := session.streams[synframe.StreamId]
            if !ok {
                debug("Opening new stream: %s\n", synframe.StreamId)
                session.streams[synframe.StreamId] = make(chan *[]byte)
                /* Send SYN_REPLY with basic headers before sending any data */
                go onRequest(session, synframe.StreamId, &synframe.Headers, session.streams[synframe.StreamId])
            } else {
                debug("Warning: peer trying to open already open stream. Dropping\n")
            }
        /* Did we receive a syn_reply control frame */
        } else if synReplyFrame, ok := frame.(*spdy.SynReplyFrame); ok {
            id := synReplyFrame.StreamId
            debug("Received syn_reply frame for stream id=%d\n", id)
            if session.Server {
                if id % 2 != 0 {
                    debug("Warning: received reply for odd-numbered stream id %d. Dropping\n", id)
                    continue
                }
            } else {
                if id % 2 == 0 {
                    debug("warning: received reply for even-numbered stream id. Dropping\n", id)
                    continue
                }
            }
            onResponse, ok := session.myStreams[id]
            if !ok {
                debug("Warning: received reply for stream id=%d but we didn't create it. Dropping\n", id)
                continue
            }
            session.streams[id] = make(chan *[]byte)
            debug("Calling response handler for stream %d\n", id)
            go onResponse(session, id, &synReplyFrame.Headers, session.streams[id])
        }
    }
}




/*
** Stream-specific operations
** (FIXME: wrap in a Stream type?)
**
*/


func (session *Session) ReplyStream(id uint32, headers http.Header, final bool) {
    var flags spdy.ControlFlags
    if final {
        debug("Setting FLAG_FIN to close stream %d\n", id)
        flags |= spdy.ControlFlagFin
    }
    session.WriteFrame(&spdy.SynReplyFrame{
        CFHeader:       spdy.ControlFrameHeader{Flags: flags},
        Headers:        headers,
        StreamId:       id,
    })
}

func attach_reader(session *Session, reader *bufio.Reader, streamId uint32, _<-chan *[]byte) {
    for {
        line, _, err := reader.ReadLine()
        if err == io.EOF {
            debug("Received EOF from input\n")
            return
        } else if err != nil {
            log.Fatal(err)
        }
        err = session.WriteFrame(&spdy.DataFrame{
            StreamId:   streamId,
            Data:       []byte(string(line)),
        })
        if err != nil {
            log.Fatal("WriteFrame: %s", err)
        }
   }
   debug("Received EOF from local input\n")
}

func attach_writer(_ *Session, writer *bufio.Writer, streamId uint32, stream <-chan *[]byte) {
    for msg := range stream {
        writer.Write(*msg)
        writer.Flush()
    }
    debug("Done reading from stream %d\n", streamId)
}

