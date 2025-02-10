package secrets

import (
	"encoding/json"
	"fmt"
	"strings"
)

func DecryptData(dec *AgeCrypt, encData, recipient string) (*Secrets, error) {
	var sec *Secrets

	if len(recipient) > 0 && dec.Recipient().String() != recipient {
		return sec, fmt.Errorf("recipient does not match provided age-key")
	}

	decData, err := dec.Decrypt(encData)
	if err != nil {
		return sec, fmt.Errorf("decrypt secrets failed: %v", err)
	}

	j := json.NewDecoder(strings.NewReader(decData))
	j.DisallowUnknownFields()
	if err = j.Decode(&sec); err != nil {
		return sec, fmt.Errorf("unmarshal secrets: invalid format data")
	}

	return sec, nil
}
