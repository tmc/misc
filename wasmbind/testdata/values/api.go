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
