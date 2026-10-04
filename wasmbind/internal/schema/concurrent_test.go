package schema

import (
	"context"
	"strings"
	"testing"
)

func TestConcurrent(t *testing.T) {
	for _, backend := range []string{"syscalljs", "abi"} {
		t.Run(backend, func(t *testing.T) {
			path := fixture(t, `package fixture
//wasmbind:resource Connection
//wasmbind:concurrent
type Connection struct{}
//wasmbind:close
func(*Connection)Close(){}
//wasmbind:export ping
func(*Connection)Ping(){}
`)
			a, err := LoadOptions(context.Background(), path, "", "", backend)
			if err != nil {
				t.Fatal(err)
			}
			if !a.Declarations[0].Concurrent {
				t.Fatal("missing concurrent policy")
			}
		})
	}
	for _, directive := range []string{"//wasmbind:record\n//wasmbind:concurrent", "//wasmbind:resource Connection\n//wasmbind:concurrent yes"} {
		path := fixture(t, "package fixture\n"+directive+"\ntype Connection struct{}\n")
		_, err := LoadOptions(context.Background(), path, "", "", "syscalljs")
		if err == nil || !strings.Contains(err.Error(), "concurrent takes no arguments and requires a resource") {
			t.Fatalf("got %v", err)
		}
	}
}

func TestConcurrentStreamControls(t *testing.T) {
	for _, concurrent := range []bool{false, true} {
		annotation := ""
		if concurrent {
			annotation = "//wasmbind:concurrent\n"
		}
		path := fixture(t, `package fixture
import "context"
//wasmbind:resource Stream
`+annotation+`type Stream struct{}
//wasmbind:read
func(*Stream)Read(context.Context,[]byte)(int,error){return 0,nil}
//wasmbind:cancelread
func(*Stream)CancelRead()error{return nil}
//wasmbind:export stop
func(*Stream)Stop(){}
//wasmbind:close
func(*Stream)Close(){}
`)
		for _, backend := range []string{"syscalljs"} {
			_, err := LoadOptions(context.Background(), path, "", "streams", backend)
			if concurrent && err != nil {
				t.Fatal(err)
			}
			if !concurrent && (err == nil || !strings.Contains(err.Error(), "ordinary method conflicts")) {
				t.Fatalf("nonconcurrent stream: %v", err)
			}
		}
	}
}
