
package myspdy

import (
    "errors"
)


/*
** A buffered message queue
*/


type MQ struct {
    messages    []interface{}
    sync        chan bool
}

func NewMQ() (*MQ) {
    return &MQ{[]interface{}{}, nil}
}

func (mq *MQ) Receive() (interface{}, error) {
    for len(mq.messages) == 0 {
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
    <-mq.sync // Block
    mq.sync = nil
    return nil
}

/*
 * Buffer a new message.
 * Warning: there is no limit to the size of the buffer.
 * If not emptied, the queue will accept new messages until it runs out of memory.
 */
func (mq *MQ) Send(msg interface{}) {
    mq.messages = append(mq.messages, msg)
    if mq.sync != nil {
        mq.sync <- true // Unblock
    }
}
