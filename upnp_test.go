// Copyright (c) 2026 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import "testing"

// TestSSDPLocation covers the header spellings real responders send.
func TestSSDPLocation(t *testing.T) {
	const loc = "http://192.168.0.1:5431/dyndev/uuid:a3aabb96-34b0-4eaf"

	tests := []struct {
		name   string
		answer string
		want   string
	}{{
		name: "no space after colon",
		answer: "HTTP/1.1 200 OK\r\nEXT:\r\nLocation:" + loc +
			"\r\nST:" + igdDeviceType + "\r\n\r\n",
		want: loc,
	}, {
		name: "canonical spacing",
		answer: "HTTP/1.1 200 OK\r\nEXT:\r\nLOCATION: " + loc +
			"\r\nST: " + igdDeviceType + "\r\n\r\n",
		want: loc,
	}, {
		name: "lowercase field names",
		answer: "HTTP/1.1 200 OK\r\nlocation: " + loc +
			"\r\nst: " + igdDeviceType + "\r\n\r\n",
		want: loc,
	}, {
		name: "another search target",
		answer: "HTTP/1.1 200 OK\r\nLocation: " + loc +
			"\r\nST: upnp:rootdevice\r\n\r\n",
		want: "",
	}, {
		name: "malformed line between the headers",
		answer: "HTTP/1.1 200 OK\r\nST: " + igdDeviceType +
			"\r\nEXT\r\nLOCATION: " + loc + "\r\n\r\n",
		want: loc,
	}, {
		name: "malformed line before the headers",
		answer: "HTTP/1.1 200 OK\r\nEXT\r\nST: " + igdDeviceType +
			"\r\nLOCATION: " + loc + "\r\n\r\n",
		want: loc,
	}, {
		name:   "no location header",
		answer: "HTTP/1.1 200 OK\r\nST: " + igdDeviceType + "\r\n\r\n",
		want:   "",
	}, {
		name:   "not a response",
		answer: "garbage",
		want:   "",
	}}

	for _, test := range tests {
		got := ssdpLocation(test.answer)
		if got != test.want {
			t.Errorf("%s: got %q, want %q", test.name, got, test.want)
		}
	}
}
