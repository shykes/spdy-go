package myspdy

import (
    "testing"
    "bytes"
    "net/http"
)


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



func TestSendStream(t *testing.T) {
    in := new(bytes.Buffer)
    out := new(bytes.Buffer)
    session, err := NewSession(out, in, nil, true)
    if err != nil {
        t.Error(err)
    }
    _, err = session.OpenStream(&http.Header{}, nil)
    if err != nil {
        t.Error(err)
    }
    if out.Len() == 0 {
        t.Errorf("No output written (should have written a SYN_STREAM)")
    }
}
