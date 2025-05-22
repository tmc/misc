package protocol

import (
	"encoding/json"
	"fmt"
)

type Request struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
}

type Notification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("JSON-RPC error %d: %s", e.Code, e.Message)
}

const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

func NewRequest(id interface{}, method string, params interface{}) *Request {
	return &Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
}

func NewNotification(method string, params interface{}) *Notification {
	return &Notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
}

func NewResponse(id interface{}, result interface{}) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}

func NewErrorResponse(id interface{}, code int, message string, data interface{}) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}

type Message interface {
	IsMessage()
}

func (r *Request) IsMessage()      {}
func (r *Response) IsMessage()     {}
func (n *Notification) IsMessage() {}

func ParseMessage(data []byte) (Message, error) {
	var temp struct {
		JSONRPC string      `json:"jsonrpc"`
		ID      interface{} `json:"id"`
		Method  string      `json:"method"`
		Result  interface{} `json:"result"`
		Error   *Error      `json:"error"`
		Params  interface{} `json:"params"`
	}

	if err := json.Unmarshal(data, &temp); err != nil {
		return nil, &Error{Code: ParseError, Message: "Parse error"}
	}

	if temp.JSONRPC != "2.0" {
		return nil, &Error{Code: InvalidRequest, Message: "Invalid Request"}
	}

	if temp.Method != "" {
		if temp.ID != nil {
			return &Request{
				JSONRPC: temp.JSONRPC,
				ID:      temp.ID,
				Method:  temp.Method,
				Params:  temp.Params,
			}, nil
		}
		return &Notification{
			JSONRPC: temp.JSONRPC,
			Method:  temp.Method,
			Params:  temp.Params,
		}, nil
	}

	return &Response{
		JSONRPC: temp.JSONRPC,
		ID:      temp.ID,
		Result:  temp.Result,
		Error:   temp.Error,
	}, nil
}