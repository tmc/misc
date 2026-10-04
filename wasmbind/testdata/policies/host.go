package policies

import (
	"context"
	"errors"
	"sync/atomic"
	"time"
)

// Identifier identifies a copied value.
//
//wasmbind:scalar Identifier
//wasmbind:validate ValidateIdentifier
type Identifier string

func ValidateIdentifier(value Identifier) error {
	if value == "yield-invalid" {
		time.Sleep(20 * time.Millisecond)
		return errors.New("invalid yielded identifier")
	}
	if value == "panic" {
		validationPanics.Add(1)
		panic("private validation panic")
	}
	if value == "slow" {
		validationStarted.Store(true)
		<-validationRelease
		validationFinished.Store(true)
	}
	if value == "yield" {
		time.Sleep(20 * time.Millisecond)
	}
	if value == "" {
		return errors.New("empty identifier")
	}
	return nil
}

//wasmbind:export identifier
func EchoIdentifier(ctx context.Context, value Identifier) Identifier {
	if value == "slow" {
		targetCalls.Add(1)
	}
	return value
}

var validationPanics atomic.Int32
var validationRelease = make(chan struct{})
var validationFinished atomic.Bool
var validationStarted atomic.Bool
var targetCalls atomic.Int32

//wasmbind:export validationPanics
func ValidationPanics() int32 { return validationPanics.Load() }

//wasmbind:export holdResource
func HoldResource(ctx context.Context, r *Resource, value Identifier, second Identifier) error {
	targetCalls.Add(1)
	return nil
}

//wasmbind:export releaseValidation
func ReleaseValidation() { close(validationRelease) }

//wasmbind:export validationFinished
func ValidationFinished() bool { return validationFinished.Load() }

//wasmbind:export validationStarted
func ValidationStarted() bool { return validationStarted.Load() }

//wasmbind:export targetCalls
func TargetCalls() int32 { return targetCalls.Load() }

//wasmbind:export invalidIdentifier
func InvalidIdentifier() Identifier { return "" }

// Host is an explicitly optional host interface.
//
//wasmbind:imports Host
//wasmbind:optional DefaultHost
type Host interface {
	// FetchURL returns a host result for identifier.
	//wasmbind:export fetch
	FetchURL(ctx context.Context, identifier Identifier) (Identifier, error)
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

func (defaultHost) FetchURL(ctx context.Context, identifier Identifier) (Identifier, error) {
	return Identifier("default:" + string(identifier)), nil
}
func DefaultHost() Host { return defaultHost{} }

var host Host

//wasmbind:init
func Initialize(ctx context.Context, h Host, empty Empty) error { host = h; return nil }

//wasmbind:export fetchHost
func FetchHost(ctx context.Context, identifier Identifier) (string, error) {
	value, err := host.FetchURL(ctx, identifier)
	return string(value), err
}
