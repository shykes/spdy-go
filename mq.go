
package myspdy

import (
    "errors"
    "io"
)


/*
** A buffered message queue
*/


type MQ struct {
    messages    []interface{}
    sync        chan bool
    closed      bool
}

func NewMQ() (*MQ) {
    return &MQ{[]interface{}{}, nil, false}
}

func (mq *MQ) Receive() (interface{}, error) {
    for len(mq.messages) == 0 {
        if mq.Closed() {
            return nil, io.EOF
        }
        err := mq.wait()
        if err != nil {
            return nil, err
        }
    }
    msg := mq.messages[0]
    mq.messages = mq.messages[1:]
    return msg, nil
}

func (mq *MQ) wait() error {
    if mq.sync != nil {
        return errors.New("MQ can only be watched by one goroutine at a time")
    }
    mq.sync = make(chan bool)
    eof := <-mq.sync // Block
    mq.sync = nil
    if eof {
        return io.EOF
    }
    return nil
}

/*
 * Buffer a new message.
 * Warning: there is no limit to the size of the buffer.
 * If not emptied, the queue will accept new messages until it runs out of memory.
 */
func (mq *MQ) Send(msg interface{}) error {
    if mq.Closed() {
        return errors.New("Can't write to closed queue")
    }
    mq.messages = append(mq.messages, msg)
    if mq.sync != nil {
        mq.sync <- false // Unblock
    }
    return nil
}

func (mq *MQ) Close() {
    mq.closed = true
    if mq.sync != nil {
        mq.sync <- true // EOF
    }
}

func (mq *MQ) Closed() bool {
    return mq.closed
}
