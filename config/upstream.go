package config

import (
	"fmt"
	"net/http"
)

type Upstream interface {
	GetServers(*http.Client) (servers []Server, err error)
	Equal(Upstream) bool
}

var ErrInvalidUpstream = fmt.Errorf("invalid upstream")
