package spdy

import (
    "bytes"
    "code.google.com/p/go.net/spdy"
    "net"
    "net/http"
    "testing"
    "time"
)

type FakeConn struct {
    bytes.Buffer
}

func (conn *FakeConn) Close() error {
    return nil
}

func (conn *FakeConn) SetDeadline(t time.Time) error {
    return nil
}

func (conn *FakeConn) SetReadDeadline(t time.Time) error {
    return nil
}

func (conn *FakeConn) SetWriteDeadline(t time.Time) error {
    return nil
}

func (conn *FakeConn) LocalAddr() net.Addr {
    return &net.TCPAddr{net.ParseIP("0.0.0.0"), 4242}
}

func (conn *FakeConn) RemoteAddr() net.Addr {
    return &net.TCPAddr{net.ParseIP("0.0.0.0"), 4242}
}

func TestDummy(t *testing.T) {
    // Do nothing
}

func TestMQ(t *testing.T) {
    mq := NewMQ()
    mq.Send("foo")
    msg, err := mq.Receive()
    if err != nil {
        t.Error("Receive() got error: %s", err)
    }
    if msg != "foo" {
        t.Error("%s != %s", msg, "foo")
    }
}

func TestMQMultipleSendOneReceive(t *testing.T) {
    mq := NewMQ()
    mq.Send("ga")
    mq.Send("bu")
    mq.Send("zo")
    mq.Send("meu")
    msg, err := mq.Receive()
    if err != nil {
        t.Errorf("Receive() got error: %s", err)
    }
    if msg != "ga" {
        t.Errorf("%s != %s", msg, "ga")
    }
    mq.Receive()
    mq.Receive()
    msg, err = mq.Receive()
    if err != nil {
        t.Errorf("Receive() got error: %s", err)
    }
    if msg != "meu" {
        t.Errorf("%s != %s", msg, "meu")
    }
}

/*
** Calling Session.OpenStream() should send a SYN_STREAM frame
 */

func TestOpenStream(t *testing.T) {
    conn := new(FakeConn)
    session, err := NewSession(conn, nil, true)
    if err != nil {
        t.Error(err)
    }
    _, err = session.OpenStream(&http.Header{})
    if err != nil {
        t.Error(err)
    }
    if conn.Len() == 0 {
        t.Errorf("No output written (should have written a SYN_STREAM)")
    }
    frame, err := session.ReadFrame()
    if err != nil {
        t.Errorf("Couldn't read frame")
    }
    if _, ok := frame.(*spdy.SynStreamFrame); !ok {
        t.Error("Couldn't read syn frame")
    }
}

/*
** Calling Stream.Output.Close() should send an empty DATA frame with FLAG_FIN=1
 */

func TestCloseStream(t *testing.T) {
    conn := new(FakeConn)
    session, err := NewSession(conn, nil, true)
    if err != nil {
        t.Error(err)
    }
    stream, err := session.OpenStream(&http.Header{})
    if err != nil {
        t.Error(err)
    }
    conn.Reset()
    stream.Output.Close()
    frame, err := session.ReadFrame()
    if err != nil {
        t.Error(err)
    }
    if dataFrame, ok := frame.(*spdy.DataFrame); ok {
        if dataFrame.StreamId != stream.Id {
            t.Errorf("Mismatched stream ID (%d != %d)", dataFrame.StreamId, stream.Id)
        }
        if dataFrame.Flags&spdy.DataFlagFin == 0 {
            t.Errorf("FLAG_FIN is not set")
        }
    } else {
        t.Error("Looked for a data frame but couldn't find one")
    }
    if conn.Len() != 0 {
        t.Errorf("Garbage frames emitted: %v", conn)
    }
}
