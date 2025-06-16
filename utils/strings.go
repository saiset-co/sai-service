package utils

import "sync"

var cache sync.Map

func Intern(buf []byte) string {
	if v, ok := cache.Load(string(buf)); ok {
		return v.(string)
	}

	s := string(buf)
	cache.Store(s, s)
	return s
}
