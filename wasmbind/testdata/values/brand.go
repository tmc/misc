package values

import (
	"fmt"
	"time"
)

//wasmbind:scalar PeerID
//wasmbind:brand
//wasmbind:validate ValidatePeerID
type PeerID string

func ValidatePeerID(v PeerID) error {
	if v == "slow" {
		time.Sleep(10 * time.Millisecond)
	}
	if v == "" {
		return fmt.Errorf("expected nonempty peer ID")
	}
	if v == "panic" {
		panic("secret")
	}
	return nil
}

//wasmbind:record
type Peers struct{ IDs []PeerID }

//wasmbind:export
func Accept(peers Peers) Peers { return peers }
