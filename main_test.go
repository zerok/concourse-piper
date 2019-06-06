package main

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

type testInfo struct {
	Title       string   `yaml:"title"`
	Details     string   `yaml:"details"`
	ExpectError bool     `yaml:"expectError"`
	Result      Pipeline `yaml:"result"`
}

func loadTestInfo(path string) (*testInfo, error) {
	var info testInfo
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(data, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

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

func TestBuildPipeline(t *testing.T) {
	files, err := ioutil.ReadDir("testcases")
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)
	require.NoError(t, err)
	for _, testcase := range files {
		t.Run(testcase.Name(), func(t *testing.T) {
			testdir := filepath.Join("testcases", testcase.Name())
			info, err := loadTestInfo(filepath.Join(testdir, "info.yml"))
			require.NoError(t, err)
			result, err := buildPipeline(context.Background(), "", testdir, false, "", log)
			if info.ExpectError && err == nil {
				t.Fatalf("\n## %s:\n\n%s\n\n! An error was expected here", info.Title, info.Details)
			}
			if !info.ExpectError && err != nil {
				t.Fatalf("\n## %s:\n\n%s\n\n! Unexpected error: %s", info.Title, info.Details, err.Error())
			}
			if err == nil {
				require.Equal(t, &info.Result, result, info.Title)
			}
			if err != nil {
				t.Fatalf("Failed to build pipeline from %s: %s", testdir, err.Error())
			}
		})
	}
}

func TestNestedPartials(t *testing.T) {
	tmpls, err := loadPartials("fixtures/nested-partials")
	assert.NoError(t, err)
	assert.NotNil(t, tmpls)
	out := &ResourceConfig{}
	logger := logrus.New()
	err = generateInstance(out, "some-instance", "some-path", []byte(`{{ partial "outer.txt" 4 . }}`), ResourceConfigHeader{}, "active-pipeline", tmpls, logger)
	assert.NoError(t, err)
	assert.Equal(t, out.Data["value"], "INNER")
}

func TestPartialsWithArgs(t *testing.T) {
	tmpls, err := loadPartials("fixtures/partials-with-args")
	assert.NoError(t, err)
	assert.NotNil(t, tmpls)
	out := &ResourceConfig{}
	logger := logrus.New()
	err = generateInstance(out, "some-instance", "some-path", []byte(`{{ partial "outer.txt" 4 . }}`), ResourceConfigHeader{}, "active-pipeline", tmpls, logger)
	assert.NoError(t, err)
}
