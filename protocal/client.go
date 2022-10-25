package protocol

import (
	"encoding/json"
	"fmt"
	"io"
	"lsp_client/comm"
	"os"
	"strconv"
	"strings"
	"sync"
)

type Client struct {
	e                    comm.Endpoint
	currWaitChan         chan *JsonrpcResponse
	currWaitID           int
	waited               bool
	asyncResponseHandler func(*JsonrpcResponse) error
}

type JsonrpcParam struct {
	JsonRpc string       `json:"jsonrpc"`
	ID      int          `json:"id"`
	Method  string       `json:"method"`
	Params  *interface{} `json:"params"`
}

type JsonrpcResponseError struct {
	Code    int          `json:"code"`
	Message string       `json:"message"`
	Data    *interface{} `json:"data"`
}

type JsonrpcResponse struct {
	ID     int                   `json:"id"`
	Result *interface{}          `json:"result"`
	Error  *JsonrpcResponseError `json:"error"`
}

var (
	idCounter    = 0
	idConterLock sync.Mutex
)

func newJsonParam(method string, params *interface{}) (int, []byte, error) {

	idConterLock.Lock()
	tmpID := idCounter
	idCounter++
	idConterLock.Unlock()

	data, err := json.Marshal(JsonrpcParam{
		JsonRpc: "2.0",
		ID:      tmpID,
		Method:  method,
		Params:  params,
	})

	prefix := []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data)))

	return tmpID, append(prefix, data...), err
}

func New(out io.Reader, err io.Reader, in io.Writer, asyncResponseHandler func(*JsonrpcResponse) error) *Client {
	ret := &Client{
		e: comm.Endpoint{
			Stdout: out,
			Stderr: err,
			Stdin:  in,
		},
		currWaitChan:         make(chan *JsonrpcResponse, 1),
		currWaitID:           0,
		waited:               false,
		asyncResponseHandler: asyncResponseHandler,
	}

	ret.e.OutHandler = ret.stdoutHandler
	ret.e.ErrHandler = ret.stderrHandler
	return ret
}

func (c *Client) stderrHandler(data []byte) (int, error) {
	return os.Stderr.Write(data)
}

func (c *Client) stdoutHandler(data []byte) (int, error) {
	s := string(data)
	done := 0
	for {
		pos := strings.Index(s, "\r\n\r\n")
		if pos == -1 {
			return done, nil
		}

		header := s[:pos]
		length, err := strconv.Atoi(strings.TrimSpace(strings.Split(header, ":")[1]))
		if err != nil {
			return done, err
		}
		if pos+4+length > len(data) {
			return done, nil
		}

		jsonData := data[done+pos+4 : done+pos+4+length]
		var rsp JsonrpcResponse
		if err := json.Unmarshal(jsonData, &rsp); err != nil {
			return done, err
		}

		if c.waited && c.currWaitID == rsp.ID {
			c.currWaitChan <- &rsp
			c.waited = false
		} else {
			if c.asyncResponseHandler != nil {
				if err := c.asyncResponseHandler(&rsp); err != nil {
					return done, err
				}
			}
		}

		done += pos + 4 + length
		s = s[pos+4+length:]
	}
}

func (c *Client) Request(method string, params *interface{}) (*JsonrpcResponse, error) {
	id, reqData, err := newJsonParam(method, params)
	if err != nil {
		return nil, err
	}

	c.currWaitID = id
	c.waited = true

	if err := c.e.Input(reqData); err != nil {
		return nil, err
	}

	ret := <-c.currWaitChan
	return ret, nil
}

func (c *Client) RequestAsync(method string, params *interface{}) error {
	_, reqData, err := newJsonParam(method, params)
	if err != nil {
		return err
	}

	return c.e.Input(reqData)
}
