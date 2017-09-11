package main

import (
	"bytes"
	"context"
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

	ctx := context.Background()

	p, err := buildPipeline(ctx, selectedPipeline, ".", wantWorldGroup, worldGroupName, log)
	if err != nil {
		log.WithError(err).Fatal("Failed to build pipeline")
	}

	if e := savePipeline(output, p); e != nil {
		log.WithError(e).Fatalf("Failed to write to %s: %s", output, e.Error())
	}

	displayPipelineStats(log, p)
}

func buildPipeline(ctx context.Context, selectedPipeline string, folder string, wantWorldGroup bool, worldGroupName string, log *logrus.Logger) (*Pipeline, error) {
	p := Pipeline{}

	partials, err := loadPartials(folder)
	if err != nil {
		return nil, fmt.Errorf("could not parse partial templates: %s", err.Error())
	}

	wg := sync.WaitGroup{}
	errorWg := sync.WaitGroup{}
	errorWg.Add(1)
	wg.Add(4)
	cancelContext, cancel := context.WithCancel(ctx)
	defer cancel()
	errChan := make(chan error, 4)
	go func() {
		defer errorWg.Done()
		for {
			select {
			case <-cancelContext.Done():
				return
			case e := <-errChan:
				fmt.Println(e)
				err = e
				cancel()
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		resources, e := loadResources(cancelContext, filepath.Join(folder, "resources"), selectedPipeline, partials, log)
		if err != nil {
			errChan <- fmt.Errorf("failed to load resources: %s", e.Error())
			return
		}
		p.Resources = resources
	}()

	go func() {
		defer wg.Done()
		resources, e := loadResources(cancelContext, filepath.Join(folder, "jobs"), selectedPipeline, partials, log)
		if e != nil {
			errChan <- fmt.Errorf("failed to load jobs: %s", e.Error())
			return
		}
		p.Jobs = resources
	}()

	go func() {
		defer wg.Done()
		resources, e := loadResources(cancelContext, filepath.Join(folder, "resource_types"), selectedPipeline, partials, log)
		if e != nil {
			errChan <- fmt.Errorf("failed to load resource_types: %s", e.Error())
			return
		}
		p.ResourceTypes = resources
	}()

	go func() {
		defer wg.Done()
		resources, e := loadResources(cancelContext, filepath.Join(folder, "groups"), selectedPipeline, partials, log)
		if err != nil {
			errChan <- fmt.Errorf("failed to load groups: %s", e.Error())
			return
		}
		p.Groups = resources
	}()

	wg.Wait()
	cancel()
	errorWg.Wait()

	if wantWorldGroup {
		worldGroup := generateWorldGroup(worldGroupName, &p)
		p.Groups = append([]Resource{worldGroup}, p.Groups...)
	}

	return &p, err
}

func savePipeline(f string, p *Pipeline) error {
	out, err := yaml.Marshal(p)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(f, out, 0644)
}

func displayPipelineStats(log *logrus.Logger, p *Pipeline) {
	log.Infof("Generated jobs (%d):", len(p.Jobs))
	for _, r := range p.Jobs {
		log.Infof(" - %s", r)
	}
	log.Infof("Generated resource_types (%d):", len(p.ResourceTypes))
	for _, r := range p.ResourceTypes {
		log.Infof(" - %s", r)
	}
	log.Infof("Generated resources (%d):", len(p.Resources))
	for _, r := range p.Resources {
		log.Infof(" - %s", r)
	}
	log.Infof("Generated groups (%d):", len(p.Groups))
	for _, r := range p.Groups {
		log.Infof(" - %s", r)
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

func ite(condition bool, trueValue interface{}, falseValue interface{}) interface{} {
	if condition {
		return trueValue
	}
	return falseValue
}

func loadResources(ctx context.Context, path string, pipeline string, partials *template.Template, log *logrus.Logger) ([]Resource, error) {
	resources := make([]Resource, 0, 10)
	if e := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if !strings.HasSuffix(p, ".yml") {
			return nil
		}
		log.Infof("Processing %s", p)
		var rc ResourceConfigHeader
		data, err := ioutil.ReadFile(p)
		if err != nil {
			return err
		}
		if err := parseHeader(&rc, data); err != nil {
			return fmt.Errorf("failed to parse header of %s: %s", p, err.Error())
		}
		if !rc.isRelevantForPipeline(pipeline) {
			return nil
		}
		for _, instance := range rc.Meta.AllInstances() {
			var instanceRC ResourceConfig
			if err := generateInstance(&instanceRC, instance, p, data, rc, pipeline, partials, log); err != nil {
				return fmt.Errorf("failed to generate instance %s: %s", instance, err.Error())
			}
			resources = append(resources, convertToResource(instanceRC, rc.Meta.Singleton()))
		}
		return nil
	}); e != nil {
		if !os.IsNotExist(e) {
			return nil, fmt.Errorf("failed to process paths: %s: %s", path, e.Error())
		}
	}
	return resources, nil
}

func generateInstance(output *ResourceConfig, instance string, path string, data []byte, input ResourceConfigHeader, activePipeline string, partials *template.Template, log *logrus.Logger) error {
	var buf bytes.Buffer
	params, ok := input.Meta.Params[instance]
	if !ok {
		params = make([]Param, 0)
	}
	log.WithField("instance", instance).Debugf("Params: %v", params)
	funcs := generateFuncMap(instance, params, partials)
	tmpl, err := template.New("ROOT").Funcs(funcs).Parse(string(data))
	if err != nil {
		log.Error(string(data))
		return fmt.Errorf("failed to parse template %s: %s", path, err.Error())
	}
	if err := tmpl.ExecuteTemplate(&buf, "ROOT", ResourceInstanceContext{
		Instance: instance,
		Params:   params,
		Pipeline: activePipeline,
	}); err != nil {
		return fmt.Errorf("failed to render template %s: %s", path, err.Error())
	}
	if err := yaml.Unmarshal(buf.Bytes(), output); err != nil {
		log.Error(buf.String())
		return fmt.Errorf("failed to unmarshal final instance config of %s (%s): %s", instance, path, err.Error())
	}
	return nil
}

func convertToResource(rc ResourceConfig, singleton bool) Resource {
	resource := Resource{}
	resource["name"] = rc.Meta.NameTemplate
	if singleton {
		resource["name"] = rc.Meta.Name
	}
	for k, v := range rc.Data {
		resource[k] = v
	}
	return resource
}

func parseHeader(rc *ResourceConfigHeader, data []byte) error {
	header, err := findHeader(data)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(header, &rc)
}

func generateFuncMap(instance string, params []Param, partials *template.Template) template.FuncMap {
	funcs := template.FuncMap{}
	funcs["getParam"] = func(name, def string) string {
		for _, p := range params {
			if p.Name == name {
				return p.Value
			}
		}
		return def
	}
	funcs["ite"] = ite
	funcs["indent"] = indent
	funcs["partial"] = func(name string, indentation int, context interface{}) (string, error) {
		var out bytes.Buffer
		if err := partials.ExecuteTemplate(&out, name, context); err != nil {
			return "", err
		}
		return indent(out.String(), indentation), nil
	}
	return funcs
}

// loadPartials optionally loads partial templates from the
// "partials" folder.
func loadPartials(path string) (*template.Template, error) {
	pat := filepath.Join(path, "partials", "*")
	files, err := filepath.Glob(pat)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return template.New("PARTIALS"), nil
	}
	return template.ParseGlob(pat)
}
