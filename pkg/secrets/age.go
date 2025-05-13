/*
 Copyright (c) 2025 Arenadata Softwer LLC.
 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package secrets

import (
	"bytes"
	"encoding/base64"
	"io"
	"strings"

	"filippo.io/age"
	"filippo.io/age/armor"
)

type AgeCrypt struct {
	*age.X25519Identity
}

func NewAgeCrypt() (*AgeCrypt, error) {
	id, err := age.GenerateX25519Identity()
	if err != nil {
		return nil, err
	}
	return &AgeCrypt{id}, nil
}

func NewAgeCryptFromString(s string) (*AgeCrypt, error) {
	id, err := age.ParseX25519Identity(s)
	if err != nil {
		return nil, err
	}
	return &AgeCrypt{id}, nil
}

func (c *AgeCrypt) EncryptValue(v string) (string, error) {
	buf := new(bytes.Buffer)
	w, err := age.Encrypt(buf, c.Recipient())
	if err != nil {
		return "", err
	}
	if _, err = w.Write([]byte(v)); err != nil {
		return "", err
	}
	if err = w.Close(); err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
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

func (c *AgeCrypt) DecryptValue(v string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return "", err
	}

	r, err := age.Decrypt(bytes.NewReader(data), c)
	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)
	if _, err = io.Copy(buf, r); err != nil {
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
