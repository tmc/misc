// Package values exercises copied input records.
//
//wasmbind:naming camelCase
package values

//wasmbind:record
type Child struct {
	Count int    `wasmbind:",optional" wasmbinddefault:"7"`
	Label string `wasmbind:",optional" wasmbinddefault:"\"default\""`
}

//wasmbind:record
type Options struct {
	Required string
	Child    Child   `wasmbind:",optional"`
	Enabled  bool    `wasmbind:",optional" wasmbinddefault:"true"`
	Nullable *Child  `wasmbind:",optional"`
	Items    []Child `wasmbind:",optional"`
	Big      uint64  `wasmbind:",optional" wasmbinddefault:"18446744073709551615"`
}

//wasmbind:export
func Echo(options Options) Options { return options }

//wasmbind:enum EventKind
//wasmbind:validate
type EventKind uint8

const (
	Ready   EventKind = 0
	Message EventKind = 1
)

//wasmbind:record
//wasmbind:union Kind
//wasmbind:variant Ready
//wasmbind:variant Message:Count,Child
type Event struct {
	Kind  EventKind
	Count int   `wasmbind:",optional" wasmbinddefault:"9"`
	Child Child `wasmbind:",optional"`
}

//wasmbind:record
type Envelope struct {
	Event Event `wasmbind:",optional"`
}

//wasmbind:export
func EventEcho(event Event) Event { return event }

//wasmbind:export
func EnvelopeEcho(envelope Envelope) Envelope { return envelope }

//wasmbind:record
type Large struct {
	Zeros [1000001]uint8 `wasmbind:",optional"`
}

//wasmbind:export
func LargeInput(large Large) bool { return true }

//wasmbind:record
type SliceChild struct {
	Bytes []byte
	Ints  []int
}

//wasmbind:record
type Slices struct {
	Bytes []byte     `wasmbind:",optional"`
	Ints  []int      `wasmbind:",optional"`
	Child SliceChild `wasmbind:",optional"`
}

//wasmbind:export
func SlicesAreNil(slices Slices) bool {
	return slices.Bytes == nil && slices.Ints == nil && slices.Child.Bytes == nil && slices.Child.Ints == nil
}

//wasmbind:export
func SlicesAreEmpty(slices Slices) bool {
	return slices.Bytes != nil && len(slices.Bytes) == 0 && slices.Ints != nil && len(slices.Ints) == 0 && slices.Child.Bytes != nil && len(slices.Child.Bytes) == 0 && slices.Child.Ints != nil && len(slices.Child.Ints) == 0
}
