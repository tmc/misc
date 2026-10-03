package echo

// Echo returns a copy of data.
//
//wasmbind:export echo
func Echo(data []byte) []byte { return append([]byte(nil), data...) }
