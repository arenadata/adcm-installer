package crypt

import (
	"bytes"
	"io"
	"strings"

	"filippo.io/age"
	"filippo.io/age/armor"
)

type AgeCrypt struct {
	*age.X25519Identity
}

func New(identities ...string) (*AgeCrypt, error) {
	var err error
	var id *age.X25519Identity

	if len(identities) > 0 && len(identities[0]) > 0 {
		id, err = age.ParseX25519Identity(identities[0])
	} else {
		id, err = age.GenerateX25519Identity()
	}
	if err != nil {
		return nil, err
	}

	return &AgeCrypt{id}, nil
}

func (c *AgeCrypt) Encrypt(data string) (string, error) {
	buf := new(bytes.Buffer)
	aw := armor.NewWriter(buf)

	w, err := age.Encrypt(aw, c.Recipient())
	if err != nil {
		return "", err
	}
	if _, err = w.Write([]byte(data)); err != nil {
		return "", err
	}
	if err = w.Close(); err != nil {
		return "", err
	}
	if err = aw.Close(); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (c *AgeCrypt) Decrypt(data string) (string, error) {
	ar := armor.NewReader(strings.NewReader(data))

	r, err := age.Decrypt(ar, c)
	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)
	if _, err = io.Copy(buf, r); err != nil {
		return "", err
	}

	return buf.String(), nil
}
