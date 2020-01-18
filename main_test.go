package main

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/spf13/afero"
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
	tests := []struct {
		name           string
		fillFS         func(afero.Fs)
		expectedResult *Pipeline
		expectedError  bool
	}{
		{
			name: "empty",
			fillFS: func(fs afero.Fs) {
				fs.Mkdir("/", 0700)
				fs.Mkdir("/jobs", 0700)
				fs.Mkdir("/resources", 0700)
				fs.Mkdir("/resource_types", 0700)
			},
			expectedResult: &Pipeline{
				Groups:        []Resource{},
				Resources:     []Resource{},
				ResourceTypes: []Resource{},
				Jobs:          []Resource{},
			},
			expectedError: false,
		}, {
			name: "simple",
			fillFS: func(fs afero.Fs) {
				fs.Mkdir("/", 0700)
				fs.Mkdir("/jobs", 0700)
				afero.WriteFile(fs, "/jobs/build.yml", []byte("meta:\n  name: build\ndata:\n"), 0600)
			},
			expectedResult: &Pipeline{
				Groups:        []Resource{},
				Resources:     []Resource{},
				ResourceTypes: []Resource{},
				Jobs: []Resource{
					{"name": "build"},
				},
			},
			expectedError: false,
		}, {
			name: "simple-with-yaml",
			fillFS: func(fs afero.Fs) {
				fs.Mkdir("/", 0700)
				fs.Mkdir("/jobs", 0700)
				afero.WriteFile(fs, "/jobs/build.yaml", []byte("meta:\n  name: build\ndata:\n"), 0600)
			},
			expectedResult: &Pipeline{
				Groups:        []Resource{},
				Resources:     []Resource{},
				ResourceTypes: []Resource{},
				Jobs: []Resource{
					{"name": "build"},
				},
			},
			expectedError: false,
		}, {
			name: "partials",
			fillFS: func(fs afero.Fs) {
				fs.Mkdir("/", 0700)
				fs.Mkdir("/jobs", 0700)
				fs.Mkdir("/partials", 0700)
				afero.WriteFile(fs, "/partials/job-def.yml", []byte("file: some-other-file-{{ getParam \"param\" \"<nil>\" }}.yml"), 0600)
				afero.WriteFile(fs, "/jobs/build.yml", []byte(`meta:
  name_template: build-{{ .Instance }}
  instances:
  - a
  - b
  params:
    a:
    - name: param
      value: a
    b:
    - name: param
      value: b
data:
  {{ partial "job-def.yml" 0 . }}`), 0600)
			},
			expectedResult: &Pipeline{
				Groups:        []Resource{},
				Resources:     []Resource{},
				ResourceTypes: []Resource{},
				Jobs: []Resource{
					{
						"name": "build-a",
						"file": "some-other-file-a.yml",
					},
					{
						"name": "build-b",
						"file": "some-other-file-b.yml",
					},
				},
			},
			expectedError: false,
		},
	}
	ctx := context.Background()
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)
	for _, testcase := range tests {
		t.Run(testcase.name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			testcase.fillFS(fs)
			result, err := buildPipeline(ctx, "", fs, "/", false, "", log)
			if testcase.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, testcase.expectedResult, result)
			}
		})
	}
}

func TestNestedPartials(t *testing.T) {
	fs := afero.NewMemMapFs()
	afero.WriteFile(fs, "/inner.txt", []byte("data:\n  value: INNER"), 0600)
	afero.WriteFile(fs, "/outer.txt", []byte("{{ partial \"inner.txt\" 0 . }}"), 0600)
	tmpls, err := loadPartials(fs, "/")
	require.NoError(t, err)
	require.NotNil(t, tmpls)
	out := &ResourceConfig{}
	logger := logrus.New()
	err = generateInstance(out, "some-instance", "some-path", []byte(`{{ partial "outer.txt" 4 . }}`), ResourceConfigHeader{}, "active-pipeline", tmpls, logger)
	require.NoError(t, err)
	require.Equal(t, out.Data["value"], "INNER")
}

func TestPartialsWithArgs(t *testing.T) {
	fs := afero.NewMemMapFs()
	afero.WriteFile(fs, "/inner.txt", []byte("data:\n  value: {{ index .Args \"value\" }}"), 0600)
	afero.WriteFile(fs, "/outer.txt", []byte("{{ partial \"inner.txt\" 0 . \"value\" \"INNER\" }}"), 0600)
	tmpls, err := loadPartials(fs, "/")
	require.NoError(t, err)
	require.NotNil(t, tmpls)
	out := &ResourceConfig{}
	logger := logrus.New()
	err = generateInstance(out, "some-instance", "some-path", []byte(`{{ partial "outer.txt" 4 . }}`), ResourceConfigHeader{}, "active-pipeline", tmpls, logger)
	require.NoError(t, err)
}
