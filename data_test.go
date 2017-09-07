package main

import "testing"

func TestResourceIsRelevantForPipeline(t *testing.T) {
	tests := []struct {
		resource ResourceConfigHeader
		pipeline string
		result   bool
		message  string
	}{
		{
			resource: ResourceConfigHeader{
				Meta: ResourceMeta{
					Pipelines: []string{},
				},
			},
			pipeline: "",
			result:   true,
			message:  "If no pipeline is specified, a resource without any pipeline should match",
		},
		{
			resource: ResourceConfigHeader{
				Meta: ResourceMeta{
					Pipelines: []string{},
				},
			},
			pipeline: "p1",
			result:   false,
			message:  "If a pipeline is requested, a resource without any pipeline shouldn't match",
		},
	}

	for _, test := range tests {
		result := test.resource.isRelevantForPipeline(test.pipeline)
		if result != test.result {
			t.Fatal(test.message)
		}
	}
}
