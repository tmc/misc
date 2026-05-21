package webkithost

import (
	"encoding/json"
	"fmt"
	"sync/atomic"

	"github.com/tmc/apple/objc"
	"github.com/tmc/apple/objectivec"
	"github.com/tmc/apple/webkit"
)

var bridgeClassCounter atomic.Uint64

// Message is one runtime-to-host bridge message.
type Message struct {
	Type         string          `json:"type"`
	Level        string          `json:"level,omitempty"`
	Message      string          `json:"message,omitempty"`
	ID           string          `json:"id,omitempty"`
	Method       string          `json:"method,omitempty"`
	Params       json.RawMessage `json:"params,omitempty"`
	Value        any             `json:"value,omitempty"`
	Capabilities map[string]bool `json:"capabilities,omitempty"`
}

// NewBridge returns a WKScriptMessageHandler that decodes JSON string messages.
func NewBridge(handle func(Message)) webkit.WKScriptMessageHandlerObject {
	className := fmt.Sprintf("GoWanixScriptBridge_%d", bridgeClassCounter.Add(1))
	protocols := []*objc.Protocol{}
	if proto := objc.GetProtocol("WKScriptMessageHandler"); proto != nil {
		protocols = append(protocols, proto)
	}
	cls, err := objc.RegisterClass(className, objc.GetClass("NSObject"), protocols, nil, []objc.MethodDef{
		{
			Cmd: objc.RegisterName("userContentController:didReceiveScriptMessage:"),
			Fn: func(self objc.ID, _ objc.SEL, controllerID objc.ID, messageID objc.ID) {
				msg := webkit.WKScriptMessageFromID(messageID)
				body := msg.Body()
				text := objectivec.ObjectFromID(body.GetID()).Description()
				var m Message
				if err := json.Unmarshal([]byte(text), &m); err != nil {
					m = Message{Type: "error", Message: "decode bridge message: " + err.Error()}
				}
				handle(m)
			},
		},
	})
	if err != nil {
		panic(fmt.Sprintf("register %s: %v", className, err))
	}
	instance := objc.Send[objc.ID](objc.ID(cls), objc.Sel("new"))
	return webkit.WKScriptMessageHandlerObjectFromID(instance)
}
