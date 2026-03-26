package utils

import (
	"sync"
	"unsafe"
)

var cache sync.Map

func Intern(buf []byte) string {
	if v, ok := cache.Load(string(buf)); ok {
		return v.(string)
	}

	s := string(buf)
	cache.Store(s, s)
	return s
}

func BytesToString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return *(*string)(unsafe.Pointer(&b))
}
