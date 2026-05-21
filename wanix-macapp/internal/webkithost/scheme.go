package webkithost

import (
	"fmt"
	"sync/atomic"

	"github.com/tmc/apple/foundation"
	"github.com/tmc/apple/objc"
	"github.com/tmc/apple/objectivec"
	"github.com/tmc/apple/webkit"
)

var schemeClassCounter atomic.Uint64

// NewSchemeHandler returns a WKURLSchemeHandler that serves assets.
func NewSchemeHandler(assets Assets) webkit.WKURLSchemeHandlerObject {
	className := fmt.Sprintf("GoWanixSchemeHandler_%d", schemeClassCounter.Add(1))
	protocols := []*objc.Protocol{}
	if proto := objc.GetProtocol("WKURLSchemeHandler"); proto != nil {
		protocols = append(protocols, proto)
	}
	cls, err := objc.RegisterClass(className, objc.GetClass("NSObject"), protocols, nil, []objc.MethodDef{
		{
			Cmd: objc.RegisterName("webView:startURLSchemeTask:"),
			Fn: func(self objc.ID, _ objc.SEL, webViewID objc.ID, taskID objc.ID) {
				task := webkit.WKURLSchemeTaskObjectFromID(taskID)
				url := task.Request().URL()
				asset, err := assets.Open(url.Path())
				if err != nil {
					task.DidReceiveResponse(foundation.NewURLResponseWithURLMIMETypeExpectedContentLengthTextEncodingName(
						url,
						"text/plain; charset=utf-8",
						len(err.Error()),
						"utf-8",
					))
					task.DidReceiveData(foundation.NewDataFromBytes([]byte(err.Error())))
					task.DidFinish()
					return
				}
				task.DidReceiveResponse(newHTTPResponse(url, 200, asset.MIME))
				task.DidReceiveData(foundation.NewDataFromBytes(asset.Data))
				task.DidFinish()
			},
		},
		{
			Cmd: objc.RegisterName("webView:stopURLSchemeTask:"),
			Fn: func(self objc.ID, _ objc.SEL, webViewID objc.ID, taskID objc.ID) {
			},
		},
	})
	if err != nil {
		panic(fmt.Sprintf("register %s: %v", className, err))
	}
	instance := objc.Send[objc.ID](objc.ID(cls), objc.Sel("new"))
	return webkit.WKURLSchemeHandlerObjectFromID(instance)
}

func newHTTPResponse(url foundation.INSURL, status int, mimeType string) foundation.NSURLResponse {
	headers := foundation.NewDictionaryWithObjectsForKeys(
		[]objectivec.IObject{
			objectivec.ObjectFromID(objc.String(mimeType)),
			objectivec.ObjectFromID(objc.String("same-origin")),
			objectivec.ObjectFromID(objc.String("require-corp")),
		},
		[]objectivec.IObject{
			objectivec.ObjectFromID(objc.String("Content-Type")),
			objectivec.ObjectFromID(objc.String("Cross-Origin-Opener-Policy")),
			objectivec.ObjectFromID(objc.String("Cross-Origin-Embedder-Policy")),
		},
	)
	resp := foundation.NewHTTPURLResponseWithURLStatusCodeHTTPVersionHeaderFields(url, status, "HTTP/1.1", headers)
	return foundation.NSURLResponseFromID(resp.GetID())
}
