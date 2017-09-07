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
	Params       map[string][]Param `yaml:"params"`
}

type ResourceConfigHeader struct {
	Meta ResourceMeta `yaml:"meta"`
}

type ResourceConfig struct {
	Meta ResourceMeta           `yaml:"meta"`
	Data map[string]interface{} `yaml:"data"`
}

type Resource map[string]interface{}

type ResourceInstanceContext struct {
	Instance string
	Params   []Param
}

type Pipeline struct {
	Groups        []Resource `yaml:"groups"`
	ResourceTypes []Resource `yaml:"resource_types"`
	Resources     []Resource `yaml:"resources"`
	Jobs          []Resource `yaml:"jobs"`
}
