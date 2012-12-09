package spdy

import (
	"errors"
	"io"
)

/*
** A buffered message queue
 */

type MQ struct {
	messages []interface{}
	sync     chan error
	closed   bool
}

func NewMQ() *MQ {
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
	mq.sync = make(chan error)
	err := <-mq.sync // Block
	mq.sync = nil
	return err
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
		mq.sync <- nil // Unblock
	}
	return nil
}

func (mq *MQ) Close() {
	mq.Error(io.EOF)
}

func (mq *MQ) Error(err error) {
	mq.closed = true
	if mq.sync != nil {
		mq.sync <- err
	}
}

func (mq *MQ) Closed() bool {
	return mq.closed
}
