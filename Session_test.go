package myspdy

import (
    "testing"
    "bytes"
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
func TestSendStream(t *testing.T) {
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
}
*/
