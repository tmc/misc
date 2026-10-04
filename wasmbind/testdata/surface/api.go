//wasmbind:naming camelCase
package api

//wasmbind:enum EventKind
//wasmbind:validate
type EventKind uint8

const (
	//wasmbind:member message
	EventMessage EventKind = 1
	//wasmbind:member lagged
	EventLagged EventKind = 2
)

//wasmbind:record
//wasmbind:union Kind
//wasmbind:variant EventMessage:Content
//wasmbind:variant EventLagged:Dropped
type Event struct {
	Kind    EventKind
	Content []byte
	Dropped uint64
}

//wasmbind:export echo
//wasmbind:namespace events
func Echo(e Event) Event { return e }

//wasmbind:export projected
//wasmbind:namespace events
func Projected() Event { return Event{Kind: EventMessage, Content: []byte("hello"), Dropped: 99} }

//wasmbind:export invalid
//wasmbind:namespace events
func Invalid() Event { return Event{Kind: 99} }

//wasmbind:resource Session
type Session struct{ value int }

//wasmbind:close
func (s *Session) Close() {}

//wasmbind:export create
//wasmbind:constructor Session
//wasmbind:nonnull
func Create() *Session { return &Session{value: 1} }

//wasmbind:export inactiveZero
//wasmbind:namespace events
func InactiveZero(e Event) bool {
	if e.Kind == EventLagged {
		return e.Content == nil
	}
	return e.Dropped == 0
}

//wasmbind:enum BigKind
//wasmbind:validate
type BigKind uint64

const (
	//wasmbind:member zero
	BigZero BigKind = 0
	//wasmbind:member max
	BigMax BigKind = 18446744073709551615
)

//wasmbind:record
//wasmbind:union Kind
//wasmbind:variant BigZero
//wasmbind:variant BigMax:Text
type BigEvent struct {
	Text string
	Kind BigKind
}

//wasmbind:export big
//wasmbind:namespace events
func Big(e BigEvent) BigEvent { return e }

// ValidatedEvent permits the Go zero of an inactive validated scalar.
//
//wasmbind:record
//wasmbind:union Kind
//wasmbind:variant EventMessage:PeerKind
//wasmbind:variant EventLagged
type ValidatedEvent struct {
	Kind     EventKind
	PeerKind EventKind
}

//wasmbind:export validated
//wasmbind:namespace events
func Validated(e ValidatedEvent) ValidatedEvent { return e }

//wasmbind:export invalidActive
//wasmbind:namespace events
func InvalidActive() ValidatedEvent { return ValidatedEvent{Kind: EventMessage} }
