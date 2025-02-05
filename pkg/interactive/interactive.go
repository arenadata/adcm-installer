package interactive

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/term"
)

type Reader func() (string, error)
type Action func() error

type Actions []Action

func (a Actions) Run() error {
	for _, act := range a {
		if err := act(); err != nil {
			return err
		}
	}
	fmt.Println("")
	return nil
}

func NewAction[T string | uint16 | int](fn Reader, prompt string, def T, out *T) Action {
	var t T
	return func() error {
		if def != t {
			prompt = fmt.Sprintf(prompt+" (default: %v)", def)
		}

		fmt.Print(prompt + ": ")

		data, err := fn()
		if err != nil {
			return err
		}

		if len(data) == 0 {
			return nil
		}

		var x any
		switch any(out).(type) {
		case *string:
			x = data
		case *int:
			x, err = strconv.Atoi(data)
		case *uint16:
			x, err = strconv.ParseUint(data, 10, 16)
		default:
			return fmt.Errorf("unknown type: %T", any(out))
		}
		if err != nil {
			return err
		}

		*out = x.(T)

		return nil
	}
}

func String(r *bufio.Reader) Reader {
	return func() (string, error) {
		data, err := r.ReadString('\n')
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(data), nil
	}
}

func Password() (string, error) {
	b, err := term.ReadPassword(syscall.Stdin)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(b)), nil
}

func File(path string) Reader {
	return func() (string, error) {
		b, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
}

func FileToBase64(path string) Reader {
	return func() (string, error) {
		b, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}

		s := base64.StdEncoding.EncodeToString(b)
		return s, nil
	}
}
