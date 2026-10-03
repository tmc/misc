// Package hello provides a small binding fixture.
package hello

// Echo returns s.
//wasmbind:export echo
func Echo(s string) string { return s }
