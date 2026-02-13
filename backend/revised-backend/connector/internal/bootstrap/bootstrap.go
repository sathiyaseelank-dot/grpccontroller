package bootstrap

import _ "embed"

//go:embed ca.crt
var caPEM []byte

func CAPEM() []byte {
	return caPEM
}
