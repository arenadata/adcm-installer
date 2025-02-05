package utils

import (
	"bufio"
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

func SplitYamlFile(file string) ([][]byte, error) {
	fi, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer fi.Close()

	scanner := bufio.NewScanner(fi)
	scanner.Split(bufio.ScanLines)

	var out [][]byte
	var buf []byte
	for scanner.Scan() {
		if scanner.Text() == "---" && len(buf) > 0 {
			out = append(out, buf)
			buf = []byte{}
			continue
		}

		b := scanner.Bytes()
		b = append(b, '\n')
		buf = append(buf, b...)
	}
	out = append(out, buf)

	return out, nil
}
