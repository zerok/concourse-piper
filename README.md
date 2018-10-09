# Concourse Piper

**Disclaimer:** This is just a very early proof-of-concept. Use at your own
risk!

If you're working with larger pipelines in concourse.ci you most likely also
have lots of similar looking jobs. This is where concourse-piper comes in. It
works on a couple of file-name conventions to generate lots of jobs using
templates.

Let's look at a small example. First: Every template used by piper follows this
structure:

```
meta:
  name_template: source-{{.Instance}}
  instances:
    - service1
    - service2
data:
  type: git
  source:
    uri: "ssh://git@your-give-server.com/ni/project.git"
    branch: develop
    private_key: ((source_repo_private_key))
    paths: ["{{.Instance}}"{{ range .Params}}{{if eq .Name "paths" }}{{.Value}}{{end}}{{end}}]
```

In general, the `meta` section defines, what resources/jobs/resource-types
should be generated and how they should be named, while in the `data` section
you describe the actual content of the file except for its name.

Piper looks for such template files in the following directories:

- jobs
- resources
- resource_types

... and merges the generated output into a single output file (which defaults to
`pipeline.generated.yml`)

## What about single jobs?

Sometimes you have jobs or resources that don't follow any template. In this
case, simply use `meta.name` insteads of `meta.name_template` and don't include
any `meta.instances`. This will generate just that one resource.

## Working with multiple pipelines?

If you're working with multiple pipelines, you can include with every template's
meta field a list of pipelines it should be included in. Now simply use the
`--pipeline` flag when launching piper to specify which pipeline should be
generated.


## Template functions

In addition to those [functions provided by Go itself](https://golang.org/pkg/text/template/#hdr-Functions)
the following functions are available within your templates:

- `getParam <name> <default>` returns the value of the first parameter matching
  the given name within the current instance.

- `ite <condition> <valueIfTrue> <valueElse>` is basically `condition ?
  valueIfTrue : valueElse`.

- `partial <name> <offset> <context>` is explained in in more detail down below.


## Partials

**Status:** Experimental

One of the goals of this project was to get rid of `task.file:` settings within
our pipelines. The issue here is, that we cannot simply deploy a new
shell-script or task configuration inside a pipeline and just test it on a
single service if that service's source does include our `ci` folder.

Using partials we can now inline the complete task configuration and deploy that
without having to touch the source repository.

Let's say we have a task which builds go applications:

```
platform: linux
image_resource:
  type: docker-image
  source:
    repository: golang
    tag: 1.9
run:
  path: /bin/bash
  args:
  - "-exc"
  - |
    go build

```

We can now store that as `partials/build-go-app.yml` and use it like that within
our pipeline:

```
jobs:
- name: build
  plan:
  - task: build
    config:
      {{ partial "build-go-app.yml" 6 . }}
```

The `6` in there is necessary as we don't know within the partial template what
level of indentation it should be rendered at in order to still procude valid
YAML.

Additionally, you can pass keyword arguments into the partial as a list with
the uneven argument being the key and its successor being the associated value:

```
{{ partial "build-go-app.yml" 6 . "some-key" "some-value" }}
```

Inside the partial the arguments are exposed through the `.Args` field which is
a `map[string]interface{}`.


## Thanks

Big thanks to [Netconomy](https://www.netconomy.net) for allowing me to work on
this :-D
