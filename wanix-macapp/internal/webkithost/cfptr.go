package webkithost

import "unsafe"

func cfPtr[T ~uintptr](v T) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&v))
}
