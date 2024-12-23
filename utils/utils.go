package utils

import (
	"math/rand"
	"os"
	"strings"
	"time"
)

func GenerateRandomString(length int) string {
	const strSrc = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0987654321@#$%^&*()_+-=[]{};:,./?~"

	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	for i := range b {
		b[i] = strSrc[rnd.Intn(len(strSrc))]
	}

	return string(b)
}

func In(a []string, s string) bool {
	for _, i := range a {
		if i == s {
			return true
		}
	}
	return false
}

func Ptr[T comparable](v T) *T {
	return &v
}

func PtrIsEmpty(v any) bool {
	switch t := v.(type) {
	case *int:
		if t == nil {
			return true
		}
		return *t == 0
	case *string:
		if t == nil {
			return true
		}
		return len(*t) == 0
	}

	return false
}

func IsPath(s string) bool {
	return len(s) == 0 || s == "." || strings.Contains(s, string(os.PathSeparator))
}

func PathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
