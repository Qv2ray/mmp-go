package config

import (
	"fmt"
)

type Upstream interface {
	GetServers() (servers []Server, err error)
	Equal(Upstream) bool
}

var ErrInvalidUpstream = fmt.Errorf("invalid upstream")
