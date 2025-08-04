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
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
)

const sep = "."

type AesCrypt struct {
	c   cipher.Block
	gcm cipher.AEAD
}

func NewAesCrypt(key []byte) (*AesCrypt, error) {
	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return nil, err
	}

	return &AesCrypt{c: c, gcm: gcm}, nil
}

func (c *AesCrypt) EncryptValue(v string) (string, error) {
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	ciphertext := c.gcm.Seal(nil, nonce, []byte(v), nil)

	return strings.Join([]string{
		base64.StdEncoding.EncodeToString(ciphertext),
		base64.StdEncoding.EncodeToString(nonce),
	}, sep), nil
}

func (c *AesCrypt) DecryptValue(v string) (string, error) {
	parts := strings.Split(v, sep)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid encrypted value format")
	}

	data, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return "", err
	}

	nonce, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", err
	}

	b, err := c.gcm.Open(nil, nonce, data, nil)
	return string(b), err
}
