package main

import "testing"

func TestFindHeader(t *testing.T) {
	tests := []struct {
		input    string
		header   []byte
		hasError bool
		message  string
	}{
		{
			input:    `nothing`,
			header:   nil,
			hasError: true,
			message:  `If no data: section can be found then an error should be returned.`,
		},
		{
			input:    "something\ndata:\nbody",
			header:   []byte(`something`),
			hasError: false,
			message:  `The header section should have been found.`,
		},
	}
	for _, test := range tests {
		header, err := findHeader([]byte(test.input))
		if string(header) != string(test.header) || (err != nil && !test.hasError) || (err == nil && test.hasError) {
			t.Logf("Found header: %s", header)
			t.Logf("Error: %v", err)
			t.Fatal(test.message)
		}
	}
}
