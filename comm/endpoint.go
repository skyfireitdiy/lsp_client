package endpoint

import (
	"fmt"
	"io"
	"os"
)

type Endpoint struct {
	Stdout     io.Reader
	Stderr     io.Reader
	Stdin      io.Writer
	OutHandler func(data []byte) (int, error)
	ErrHandler func(data []byte) (int, error)
}

func (e *Endpoint) Run() {
	resolver := func(r io.Reader, cb func(data []byte) (int, error)) {
		tmpBuf := make([]byte, 4096)
		innerBuf := []byte{}
		for {
			n, err := r.Read(tmpBuf)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return
			}
			innerBuf = append(innerBuf, tmpBuf[:n]...)
			if cb == nil {
				continue
			}
			r, err := cb(innerBuf)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return
			}
			innerBuf = innerBuf[r:]
		}

	}
	if e.Stdout != nil {
		go resolver(e.Stdout, e.OutHandler)
	}
	if e.Stderr != nil {
		go resolver(e.Stderr, e.ErrHandler)
	}
}

func (e *Endpoint) Input(data []byte) error {
	l := len(data)
	sent := 0
	for sent != l {
		n, err := e.Stdin.Write(data[sent:])
		if err != nil {
			return err
		}
		sent += n
	}
	return nil
}
