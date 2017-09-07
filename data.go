package main

type Param struct {
	Name    string `yaml:"name"`
	Value   string `yaml:"value"`
	Section string `yaml:"section"`
}

type ResourceMeta struct {
	Name         string             `yaml:"name"`
	NameTemplate string             `yaml:"name_template"`
	Instances    []string           `yaml:"instances"`
	Pipelines    []string           `yaml:"pipelines"`
	Params       map[string][]Param `yaml:"params"`
}

type ResourceConfigHeader struct {
	Meta ResourceMeta `yaml:"meta"`
}

func (r *ResourceConfigHeader) isRelevantForPipeline(pipeline string) bool {
	if r.Meta.Pipelines == nil || len(r.Meta.Pipelines) == 0 {
		return pipeline == ""
	}
	for _, p := range r.Meta.Pipelines {
		if p == pipeline {
			return true
		}
	}
	return false
}

type ResourceConfig struct {
	Meta ResourceMeta           `yaml:"meta"`
	Data map[string]interface{} `yaml:"data"`
}

type Resource map[string]interface{}

type ResourceInstanceContext struct {
	Instance string
	Params   []Param
	Pipeline string
}

type Pipeline struct {
	Groups        []Resource `yaml:"groups"`
	ResourceTypes []Resource `yaml:"resource_types"`
	Resources     []Resource `yaml:"resources"`
	Jobs          []Resource `yaml:"jobs"`
}
