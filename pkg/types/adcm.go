package types

import "encoding/json"

type DbSSLOptions struct {
	SSLMode     string `json:"sslmode"`
	SSLCert     string `json:"sslcert,omitempty"`
	SSLKey      string `json:"sslkey,omitempty"`
	SSLRootCert string `json:"sslrootcert,omitempty"`
}

func (opt DbSSLOptions) String() string {
	b, err := json.Marshal(opt)
	if err != nil {
		panic(err)
	}
	return string(b)
}
