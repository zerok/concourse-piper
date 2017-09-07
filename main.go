package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/spf13/pflag"

	"text/template"

	"gopkg.in/yaml.v2"
)

func findHeader(data []byte) ([]byte, error) {
	idx := bytes.Index(data, []byte("data:\n"))
	if idx == -1 {
		return nil, fmt.Errorf("could not find header")
	}
	return data[0 : idx-1], nil
}

func attachTemplates(t *template.Template, folder string) error {
	return filepath.Walk(folder, func(p string, info os.FileInfo, err error) error {
		if !strings.HasSuffix(p, ".yml") {
			return nil
		}
		name := strings.TrimSuffix(p, ".yml")
		data, err := ioutil.ReadFile(p)
		if err != nil {
			return err
		}
		_, err = t.New(name).Parse(string(data))
		return err
	})
}

func main() {
	var output string
	var verbose bool
	var worldGroupName string
	var wantWorldGroup bool
	pflag.StringVar(&output, "output", "pipeline.generated.yaml", "Path to an output file for the generated pipeline")
	pflag.BoolVar(&wantWorldGroup, "worldgroup", false, "Generate a group containing all resources and jobs")
	pflag.StringVar(&worldGroupName, "worldgroup-name", "WORLD", "Name of the group that contains all jobs and resources")
	pflag.BoolVar(&verbose, "verbose", false, "Verbose logging")
	pflag.Parse()
	log := logrus.New()
	if verbose {
		log.SetLevel(logrus.DebugLevel)
	}

	p := Pipeline{}

	wg := sync.WaitGroup{}
	wg.Add(4)

	go func() {
		resources, err := loadResources("resources", log)
		if err != nil {
			log.WithError(err).Fatal("Failed to load resources")
		}
		p.Resources = resources
		wg.Done()
	}()

	go func() {
		resources, err := loadResources("jobs", log)
		if err != nil {
			log.WithError(err).Fatal("Failed to load jobs")
		}
		p.Jobs = resources
		wg.Done()
	}()

	go func() {
		resources, err := loadResources("resource_types", log)
		if err != nil {
			log.WithError(err).Fatal("Failed to load resource_types")
		}
		p.ResourceTypes = resources
		wg.Done()
	}()

	go func() {
		resources, err := loadResources("groups", log)
		if err != nil {
			log.WithError(err).Fatal("Failed to load groups")
		}
		p.Groups = resources
		wg.Done()
	}()

	wg.Wait()

	if wantWorldGroup {
		worldGroup := generateWorldGroup(worldGroupName, &p)
		p.Groups = append([]Resource{worldGroup}, p.Groups...)
	}

	out, err := yaml.Marshal(p)
	if err != nil {
		log.WithError(err).Fatal("Failed to encode pipeline")
	}

	if err := ioutil.WriteFile(output, out, 0644); err != nil {
		log.WithError(err).Fatalf("Failed to write to %s", output)
	}

	log.Infof("Generated jobs (%d):", len(p.Jobs))
	for _, r := range p.Jobs {
		log.Infof(" - %s", r["name"])
	}
	log.Infof("Generated resource_types (%d):", len(p.ResourceTypes))
	for _, r := range p.ResourceTypes {
		log.Infof(" - %s", r["name"])
	}
	log.Infof("Generated resources (%d):", len(p.Resources))
	for _, r := range p.Resources {
		log.Infof(" - %s", r["name"])
	}
	log.Infof("Generated groups (%d):", len(p.Groups))
	for _, r := range p.Groups {
		log.Infof(" - %s", r["name"])
	}
}

func generateWorldGroup(name string, p *Pipeline) Resource {
	r := Resource{}
	jobNames := make([]string, 0, len(p.Jobs))
	for _, j := range p.Jobs {
		jobNames = append(jobNames, j["name"].(string))
	}
	resourceNames := make([]string, 0, len(p.Resources))
	for _, r := range p.Resources {
		resourceNames = append(resourceNames, r["name"].(string))
	}
	r["name"] = name
	r["jobs"] = jobNames
	r["resources"] = resourceNames
	return r
}

func loadResources(path string, log *logrus.Logger) ([]Resource, error) {
	resources := make([]Resource, 0, 10)
	if err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		log.Infof("Processing %s", p)
		var rc ResourceConfigHeader
		if !strings.HasSuffix(p, ".yml") {
			return nil
		}
		data, err := ioutil.ReadFile(p)
		if err != nil {
			return err
		}
		header, err := findHeader(data)
		if err != nil {
			log.Error(string(data))
			return err
		}
		if err := yaml.Unmarshal(header, &rc); err != nil {
			log.Error(string(data))
			return err
		}
		var useName bool
		if len(rc.Meta.Instances) == 0 && rc.Meta.Name != "" {
			rc.Meta.Instances = []string{rc.Meta.Name}
			useName = true
		}
		for _, instance := range rc.Meta.Instances {
			var output bytes.Buffer
			var params []Param
			ps, ok := rc.Meta.Params[instance]
			if ok {
				params = ps
			} else {
				params = make([]Param, 0, 0)
			}
			log.WithField("instance", instance).Debug(params)
			funcs := template.FuncMap{}
			funcs["getParam"] = func(name, def string) string {
				for _, p := range params {
					if p.Name == name {
						return p.Value
					}
				}
				return def
			}
			tmpl, err := template.New("ROOT").Funcs(funcs).Parse(string(data))
			if err != nil {
				return err
			}
			if err := attachTemplates(tmpl, "tasks"); err != nil {
				return err
			}
			if err := tmpl.ExecuteTemplate(&output, "ROOT", ResourceInstanceContext{
				Instance: instance,
				Params:   params,
			}); err != nil {
				return err
			}
			var instanceRC ResourceConfig
			if err := yaml.Unmarshal(output.Bytes(), &instanceRC); err != nil {
				log.Error(output.String())
				return err
			}
			resource := Resource{}
			if useName {
				resource["name"] = instanceRC.Meta.Name
			} else {
				resource["name"] = instanceRC.Meta.NameTemplate
			}
			for k, v := range instanceRC.Data {
				resource[k] = v
			}
			resources = append(resources, resource)
		}

		return nil
	}); err != nil {
		return nil, err
	}
	return resources, nil
}
