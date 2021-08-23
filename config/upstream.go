package config

import (
	"fmt"
	"reflect"
)

type Upstream interface {
	GetName()    string
	GetServers() (servers []Server, err error)
}

var InvalidUpstreamErr = fmt.Errorf("invalid upstream")

func Map2Upstream(m UpstreamConf, upstream interface{}) error {
	v := reflect.ValueOf(upstream)
	if !v.IsValid() {
		return fmt.Errorf("upstream should not be nil")
	}
	v = v.Elem()
	if !v.IsValid() {
		return fmt.Errorf("upstream should be a pointer")
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag
		key := tag.Get("json")
		vf := v.Field(i)
		vf.SetString(m[key].(string))
	}
	return nil
}
