package main

import "fmt"

// Param is a parameter which can be applied to an instance
// during the template-execution phase.
type Param struct {
	Name    string `yaml:"name"`
	Value   string `yaml:"value"`
	Section string `yaml:"section"`
}

// ResourceMeta represents the header of a resource template
// defining what instances of the resource should be
// generated.
type ResourceMeta struct {
	Name         string             `yaml:"name"`
	NameTemplate string             `yaml:"name_template"`
	Instances    []string           `yaml:"instances"`
	Pipelines    []string           `yaml:"pipelines"`
	Params       map[string][]Param `yaml:"params"`
}

// Singleton returns true if no instances are configured.
func (m *ResourceMeta) Singleton() bool {
	return m.Instances == nil || len(m.Instances) == 0
}

// AllInstances returns the list of instances configured. If none are
// defined, a singleton-list containing .Name will be returned.
func (m *ResourceMeta) AllInstances() []string {
	if m.Singleton() {
		return []string{m.Name}
	}
	return m.Instances
}

// ResourceConfigHeader represents the header of a resource
// file containing just the `meta`-section.
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

// ResourceConfig is the content of a resource template file.
type ResourceConfig struct {
	Meta ResourceMeta           `yaml:"meta"`
	Data map[string]interface{} `yaml:"data"`
}

// Resource is either a job, resource, etc. after the template
// execution and YAML unmarshalling step.
type Resource map[string]interface{}

func (r Resource) String() string {
	s, ok := r["name"].(string)
	if !ok {
		return fmt.Sprintf("<%v>", r["name"])
	}
	return s
}

// ResourceInstanceContext is the  context available during the
// template-execution phase.
type ResourceInstanceContext struct {
	Instance string
	Params   []Param
	Pipeline string
	Args     map[string]interface{}
}

func (rc *ResourceInstanceContext) Clone() ResourceInstanceContext {
	params := make([]Param, 0, len(rc.Params))
	for _, p := range rc.Params {
		params = append(params, p)
	}
	return ResourceInstanceContext{
		Pipeline: rc.Pipeline,
		Params:   params,
		Instance: rc.Instance,
	}
}

// Pipeline is the data structure used for rendering out the
// output document.
type Pipeline struct {
	Groups        []Resource `yaml:"groups"`
	ResourceTypes []Resource `yaml:"resource_types"`
	Resources     []Resource `yaml:"resources"`
	Jobs          []Resource `yaml:"jobs"`
}
