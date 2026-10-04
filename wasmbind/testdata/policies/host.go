package policies

import (
	"context"
	"errors"
)

// Identifier identifies a copied value.
//
//wasmbind:scalar Identifier
//wasmbind:validate ValidateIdentifier
type Identifier string

func ValidateIdentifier(value Identifier) error {
	if value == "" {
		return errors.New("empty identifier")
	}
	return nil
}

//wasmbind:export identifier
func EchoIdentifier(value Identifier) Identifier { return value }

//wasmbind:export invalidIdentifier
func InvalidIdentifier() Identifier { return "" }

// Host is an explicitly optional host interface.
//
//wasmbind:imports Host
//wasmbind:optional DefaultHost
type Host interface {
	// FetchURL returns a host result for identifier.
	//wasmbind:export fetch
	FetchURL(ctx context.Context, identifier Identifier) (string, error)
}

// Empty exercises optional import presence without any methods.
//
//wasmbind:imports Empty
//wasmbind:optional DefaultEmpty
type Empty interface{}

var emptyDefaults int

func DefaultEmpty() Empty { emptyDefaults++; return struct{}{} }

//wasmbind:export emptyDefaults
func EmptyDefaults() int { return emptyDefaults }

type defaultHost struct{}

func (defaultHost) FetchURL(ctx context.Context, identifier Identifier) (string, error) {
	return "default:" + string(identifier), nil
}
func DefaultHost() Host { return defaultHost{} }

var host Host

//wasmbind:init
func Initialize(ctx context.Context, h Host, empty Empty) error { host = h; return nil }

//wasmbind:export fetchHost
func FetchHost(ctx context.Context, identifier Identifier) (string, error) {
	return host.FetchURL(ctx, identifier)
}
