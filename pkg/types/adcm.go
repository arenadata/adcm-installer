package types

import "encoding/json"

type DbSSLOptions struct {
	SSLMode     string `json:"sslmode"`
	SSLCert     string `json:"sslcert"`
	SSLKey      string `json:"sslkey"`
	SSLRootCert string `json:"sslrootcert"`
}

func (opt DbSSLOptions) String() string {
	b, err := json.Marshal(opt)
	if err != nil {
		panic(err)
	}
	return string(b)
}
