package generate

import (
	"strings"
	"testing"

	"github.com/tmc/misc/wasmbind/internal/schema"
)

func TestConcurrentReceiver(t *testing.T) {
	for _, backend := range []string{"syscalljs", "abi"} {
		t.Run(backend, func(t *testing.T) {
			for _, concurrent := range []bool{false, true} {
				a := &schema.API{Manifest: schema.Manifest{Backend: backend, Package: "example.com/api", Declarations: []*schema.Decl{{Kind: "resource", Name: "Conn", GoName: "Conn", Close: "Close", Concurrent: concurrent}}, Calls: []*schema.Call{{ID: 1, Name: "ping", GoName: "Ping", Receiver: "Conn", ReceiverGo: "Conn"}, {ID: 2, Name: "$read", GoName: "Read", Receiver: "Conn", ReceiverGo: "Conn", Op: "read"}}}, Imports: map[string]string{"example.com/api": "api"}}
				files, err := Files(a)
				if err != nil {
					t.Fatal(err)
				}
				var source string
				for name, data := range files {
					if strings.HasSuffix(name, ".go") {
						source += string(data)
					}
				}
				if strings.Contains(source, `b.queue(r, i, "")`) == concurrent {
					t.Fatal("ordinary method queue did not match concurrency policy")
				}
				if !strings.Contains(source, `b.queue(r, i, "read")`) {
					t.Fatal("concurrency removed the read lane")
				}
			}
		})
	}
}
