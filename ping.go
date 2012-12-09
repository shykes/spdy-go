package spdy

import (
    "code.google.com/p/go.net/spdy"
    "time"
)

/*
** type Ping
**
** A record of PING frames received or sent.
** (See http://tools.ietf.org/html/draft-mbelshe-httpbis-spdy-00#section-2.6.5)
 */

type Ping struct {
    Id    uint32
    Start time.Time
    RTT   time.Duration
}

func (session *Session) handlePingFrame(pingFrame *spdy.PingFrame) error {
    id := pingFrame.Id
    if !session.isLocalId(id) { // Peer is opening a new ping
        _, exists := session.pings[id]
        if exists { // Already received this ping. Ignore.
            debug("Warning: received duplicate ping. Ignoring.\n")
            return nil
        }
        err := session.WriteFrame(pingFrame) // Right back at ya
        if err != nil {
            return err
        }
        session.pings[id] = &Ping{Id: id}
    } else { // Peer is responding to a ping
        id := pingFrame.Id
        ping, exists := session.pings[id]
        if !exists {
            debug("warning: received response to unknown ping. Ignoring.\n")
            return nil
        }
        if ping.RTT != 0 {
            debug("Warning: received duplicate response to ping %d. Ignoring.\n", id)
            return nil
        }
        ping.RTT = time.Now().Sub(ping.Start)
        debug("Ping RTT=%v\n", ping.RTT)
    }
    return nil
}

/*
** Send a new ping frame
 */

func (session *Session) Ping() error {
    ping := Ping{
        Id:    session.nextId(session.lastPingId),
        Start: time.Now(),
    }
    debug("Sending PING id=%v\n", ping.Id)
    session.lastPingId = ping.Id
    session.pings[ping.Id] = &ping
    err := session.WriteFrame(&spdy.PingFrame{Id: ping.Id})
    if err != nil {
        debug("[Ping] writeframe failed\n")
        return err
    }
    return nil
}

/*
** Send a ping every 30 seconds
 */

func (session *Session) pingLoop() error {
    for {
        err := session.Ping()
        if err != nil {
            debug("[pingLoop] ping failed\n")
            return err
        }
        time.Sleep(30 * time.Second)
    }
    return nil
}
