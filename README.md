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

## Thanks

Big thanks to [Netconomy](https://www.netconomy.net) for allowing me to work on
this :-D
