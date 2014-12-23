package main

import "strings"

type stringsl []string

func (s *stringsl) String() string {
	return strings.Join(*s, ",")
}

func (s *stringsl) Set(val string) error {
	*s = append(*s, val)
	return nil
}
