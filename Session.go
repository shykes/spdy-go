

package spdy

import (
    "code.google.com/p/go.net/spdy"
    "net"
    "net/http"
)


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
    conn                net.Conn
    lastPingId          uint32                  // Last (and highest-numbered) ping ID we allocated
    pings               map[uint32] *Ping
}


/* Create a new Session object */
func NewSession(conn net.Conn, handler Handler, server bool) (*Session, error) {
    framer, err := spdy.NewFramer(conn, conn)
    if err != nil {
        return nil, err
    }
    return &Session{
        framer,
        server,
        0,
        make(map[uint32]*Stream),
        handler,
        conn,
        0,
        make(map[uint32]*Ping),
    }, nil
}



func (session *Session) Close() error {
    debug("Session.Close()\n")
    return session.conn.Close()
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
 func (session *Session) nextId(lastId uint32) uint32 {
    if lastId == 0 {
        if session.Server {
            return 2
        } else {
            return 1
        }
    }
    // FIXME: optionally return an error on wrap
    // (ping IDs are allowed to wrap, but stream IDs aren't)
    return lastId + 2
}


func (session *Session) OpenStream(headers *http.Header) (*Stream, error) {
    newId := session.nextId(session.lastStreamId)
    stream := newStream(session, newId, true)
    session.lastStreamId = newId
    session.streams[newId] = stream
    updateHeaders(stream.Output.Headers(), headers)
    err := stream.Output.SendHeaders(false)
    if err != nil {
        return nil, err
    }
    return stream, nil
}


/*
** Session main loop: run the receive loop and ping loop in parallel,
** and return an error if either one fails.
*/


func (session *Session) Run() error {
    debug("Session.Run()")
    pingChan := Promise(func() error { return session.pingLoop() })
    receiveChan := Promise(func() error { return session.receiveLoop() })
    for {
        select {
            case err := <-pingChan: {
                if err != nil {
                    debug("Ping failed! Interrupting run loop\n")
                    session.Close()
                    return err
                }
            }
            case err := <-receiveChan: {
                if err != nil {
                    debug("Receive failed: %s. Interrupting run loop\n", err)
                    session.Close()
                    return err
                }
            }
        }
    }
    return nil
}


/*
** Listen for new frames and process them
*/

func (session *Session) receiveLoop() error {
    debug("Starting receive loop\n")
    for {
        frame, err := session.ReadFrame()
        if err != nil {
            for _, stream := range session.streams {
                stream.Input.Error(err)
            }
            return err
        }
        debug("Received frame %s\n", frame)
        /* Did we receive a data frame? */
        if dframe, ok := frame.(*spdy.DataFrame); ok {
            stream, exists := session.streams[dframe.StreamId]
            if !exists {
                // Protocol error
                debug("Received a data frame for unknown stream id %s. Dropping.\n", dframe.StreamId)
                continue
            }
            stream.Input.Push(&dframe.Data)
            if dframe.Flags & spdy.DataFlagFin != 0 {
                stream.Input.Close()
            }

        /* FIXME: Did we receive a headers control frame? */
        } else if headersframe, ok := frame.(*spdy.HeadersFrame); ok {
            stream, exists := session.streams[headersframe.StreamId]
            if !exists { // Protocol error
                debug("Received headers for unknown stream id %s. Dropping.\n", headersframe.StreamId)
                continue
            }
            updateHeaders(stream.Input.Headers(), &headersframe.Headers)
            // FIXME: notify of new headers?
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
            if synframe.CFHeader.Flags & spdy.ControlFlagFin != 0 {
                stream.Input.Close()
            }
            /* Run the handler */
            go func() {
                session.handler.ServeSPDY(stream)
                stream.Output.Close()
            }()
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
            /* If FLAG_FIN is set, half-close the stream */
            if synReplyFrame.CFHeader.Flags & spdy.ControlFlagFin != 0 {
                stream.Input.Close()
            }
        } else if pingFrame, ok := frame.(*spdy.PingFrame); ok {
            if err := session.handlePingFrame(pingFrame); err != nil {
                return err
            }
        }
    }
    return nil
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
