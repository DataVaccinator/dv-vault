package main

import "fmt"

/*
This package contains the function that replaces the generic
logger used by go net/http. The reason is the need to filter a
few messages before writing to stdout. Otherwise, the log is
filled up with useless information about scanners and attackers
bad SSL usage behaviour.

It mainly filters all log entries beginning with
"echo: http: TLS handshake error from ..."

The rest is simply printed to stdout (ordinary log writer).
*/

// Filter all message sthat start with this string
const toFilter = "echo: http: TLS handshake error from"

type filterLogger struct {
}

// Write implements the io.Writer functionality to be a replacement
// for the net/http logger.
func (w *filterLogger) Write(data []byte) (int, error) {
	out := string(data)
	if out[0:len(toFilter)] == toFilter {
		// Do not log this sort of messages
		return len(data), nil
	}
	fmt.Print(out)
	return len(data), nil
}
