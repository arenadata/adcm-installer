package utils

import (
	"fmt"
	"math/rand"
	"os"
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

func Ptr[T comparable](v T) *T {
	return &v
}

func PtrIsEmpty[T comparable](v *T) bool {
	if v == nil {
		return true
	}

	var t T
	return *v == t
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
