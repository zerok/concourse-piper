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

var version, commit, date string

func findHeader(data []byte) ([]byte, error) {
	idx := bytes.Index(data, []byte("data:\n"))
	if idx == -1 {
		return nil, fmt.Errorf("could not find header")
	}
	return data[0 : idx-1], nil
}

func main() {
	var output string
	var verbose bool
	var worldGroupName string
	var wantWorldGroup bool
	var selectedPipeline string
	var showVersion bool
	pflag.StringVar(&output, "output", "pipeline.generated.yaml", "Path to an output file for the generated pipeline")
	pflag.BoolVar(&wantWorldGroup, "worldgroup", false, "Generate a group containing all resources and jobs")
	pflag.StringVar(&worldGroupName, "worldgroup-name", "WORLD", "Name of the group that contains all jobs and resources")
	pflag.BoolVar(&verbose, "verbose", false, "Verbose logging")
	pflag.StringVar(&selectedPipeline, "pipeline", "", "Specify the name of the pipeline you want to generate")
	pflag.BoolVar(&showVersion, "version", false, "Show version information")
	pflag.Parse()
	log := logrus.New()
	if verbose {
		log.SetLevel(logrus.DebugLevel)
	}
	if showVersion {
		fmt.Printf("Version: %s\nCommit: %s\nDate: %s\n", version, commit, date)
		os.Exit(0)
	}

	p := Pipeline{}

	partials, err := template.ParseGlob("partials/*")
	if err != nil {
		log.WithError(err).Fatal("Could not parse partial templates")
	}

	wg := sync.WaitGroup{}
	wg.Add(4)

	go func() {
		resources, err := loadResources("resources", selectedPipeline, partials, log)
		if err != nil {
			log.WithError(err).Fatal("Failed to load resources")
		}
		p.Resources = resources
		wg.Done()
	}()

	go func() {
		resources, err := loadResources("jobs", selectedPipeline, partials, log)
		if err != nil {
			log.WithError(err).Fatal("Failed to load jobs")
		}
		p.Jobs = resources
		wg.Done()
	}()

	go func() {
		resources, err := loadResources("resource_types", selectedPipeline, partials, log)
		if err != nil {
			log.WithError(err).Fatal("Failed to load resource_types")
		}
		p.ResourceTypes = resources
		wg.Done()
	}()

	go func() {
		resources, err := loadResources("groups", selectedPipeline, partials, log)
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

func indent(data string, offset int) string {
	lines := make([]string, 0, 5)
	for idx, line := range strings.Split(data, "\n") {
		if idx == 0 {
			lines = append(lines, line)
			continue
		}
		lines = append(lines, fmt.Sprintf("%s%s", strings.Repeat(" ", offset), line))
	}
	return strings.Join(lines, "\n")
}

func loadResources(path string, pipeline string, partials *template.Template, log *logrus.Logger) ([]Resource, error) {
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
			return fmt.Errorf("failed to unmarshal header of %s: %s", p, err)
		}
		var useName bool
		if len(rc.Meta.Instances) == 0 && rc.Meta.Name != "" {
			rc.Meta.Instances = []string{rc.Meta.Name}
			useName = true
		}
		if !rc.isRelevantForPipeline(pipeline) {
			return nil
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
			funcs["ite"] = func(condition bool, trueValue interface{}, falseValue interface{}) interface{} {
				if condition {
					return trueValue
				}
				return falseValue
			}
			funcs["indent"] = indent
			funcs["partial"] = func(name string, indentation int, context interface{}) (string, error) {
				var out bytes.Buffer
				if err := partials.ExecuteTemplate(&out, name, context); err != nil {
					return "", err
				}
				return indent(out.String(), indentation), nil
			}
			tmpl, err := template.New("ROOT").Funcs(funcs).Parse(string(data))
			if err != nil {
				return fmt.Errorf("failed to parse template %s: %s", p, err.Error())
			}
			if err := tmpl.ExecuteTemplate(&output, "ROOT", ResourceInstanceContext{
				Instance: instance,
				Params:   params,
				Pipeline: pipeline,
			}); err != nil {
				return fmt.Errorf("failed to render template %s: %s", p, err.Error())
			}
			var instanceRC ResourceConfig
			if err := yaml.Unmarshal(output.Bytes(), &instanceRC); err != nil {
				log.Error(output.String())
				return fmt.Errorf("failed to unmarshal final instance config of %s (%s): %s", instance, p, err.Error())
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
		return nil, fmt.Errorf("failed to process paths: %s: %s", path, err.Error())
	}
	return resources, nil
}
