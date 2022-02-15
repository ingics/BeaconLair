package main

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// Go routine identity (experimental).
// Golang hide routine identity for reasons, but I like it for debug logging
func GoID() int {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	idField := strings.Fields(strings.TrimPrefix(string(buf[:n]), "goroutine "))[0]
	id, err := strconv.Atoi(idField)
	if err != nil {
		panic(fmt.Sprintf("cannot get goroutine id: %v", err))
	}
	return id
}

func getEnvIntVar(key string, def int) int {
	if val_str, ok := os.LookupEnv(key); ok {
		if val, err := strconv.Atoi(val_str); err != nil {
			fmt.Printf("Invalid env var %s: %s\n", key, val_str)
		} else {
			return val
		}
	}
	return def
}
