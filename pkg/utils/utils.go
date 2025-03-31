package utils

import (
	"fmt"
	"math/rand/v2"
	"os"
)

func GenerateRandomString(length int) string {
	const strSrc = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0987654321@#$%^&*()_+-=[]{};:,./?~"

	b := make([]byte, length)
	for i := range b {
		b[i] = strSrc[rand.IntN(len(strSrc))]
	}

	return string(b)
}

func Ptr[T comparable](v T) *T {
	return &v
}

func FileExists(path string) (bool, error) {
	st, err := os.Stat(path)
	if err != nil {
		return false, nil
	}
	if st.IsDir() {
		return false, fmt.Errorf("%s is a directory", path)
	}

	return true, nil
}

func In(a []string, s string) bool {
	for _, i := range a {
		if i == s {
			return true
		}
	}
	return false
}
