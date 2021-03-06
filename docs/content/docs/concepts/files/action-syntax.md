---
title: "Action configuration file"
weight: 3
card: 
  name: concept_pipeline
  weight: 3
---

Hello World Action

```yml
version: v1.0
name: CDS_HelloWorld
description: Hello World Action
steps:
- name: Initialization
  script:
  - echo "Hello World"
```

With a real action `CDS_SonarScanner`: this action contains
parameters with default values and some of them are `advanced` parameters. 
Two plugins are also used in the steps: `plugin-download` and `plugin-archive`

```yml
version: v1.0
name: CDS_SonarScanner
description: Run Sonar analysis. You must have a file sonar-project.properties in
  your source directory.
parameters:
  sonar-project.properties:
    type: text
    default: |-
      sonar.projectKey={{.cds.application}}
      sonar.projectName={{.cds.application}}
      sonar.projectVersion={{.git.hash}}
      sonar.sources=.
      sonar.exclusions=**/*_test.go,**/vendor/**
      sonar.tests=.
      sonar.test.inclusions=**/*_test.go
      sonar.test.exclusions=**/vendor/**
    description: sonar-project.properties file
  sonarBranch:
    type: string
    default: '{{.git.branch}}'
    description: The Sonar branch (e.g. master)
  sonarDownloadURL:
    type: string
    default: https://binaries.sonarsource.com/Distribution/sonar-scanner-cli/sonar-scanner-cli-{{.sonarVersion}}-linux.zip
    description: The download URL of Sonar CLI
    advanced: true
  sonarPassword:
    type: string
    default: '{{.cds.proj.sonarPassword}}'
    description: The Sonar server's password
    advanced: true
  sonarURL:
    type: string
    default: '{{.cds.proj.sonarURL}}'
    description: The URL of the Sonar server
    advanced: true
  sonarUsername:
    type: string
    default: '{{.cds.proj.sonarUsername}}'
    description: The Sonar server's username
    advanced: true
  sonarVersion:
    type: string
    default: 3.2.0.1227
    description: SonarScanner's version to use
    advanced: true
  workspace:
    type: string
    default: '{{.cds.workspace}}'
    description: The directory where your project is (e.g. /go/src/github.com/ovh/cds)
requirements:
- binary: bash
- plugin: plugin-archive
- plugin: plugin-download
steps:
- name: Initialization
  script:
  - '#!/bin/bash'
  - set -x
  - '# Installation'
  - mkdir -p {{.workspace}}/opt
- plugin-download:
    filepath: '{{.workspace}}/opt/sonar-scanner-cli-{{.sonarVersion}}-linux.zip'
    url: '{{.sonarDownloadURL}}'
- plugin-archive:
    action: uncompress
    destination: '{{.workspace}}/opt/'
    source: '{{.workspace}}/opt/sonar-scanner-cli-{{.sonarVersion}}-linux.zip'
- script:
  - '#!/bin/bash'
  - set -x
  - ""
  - '# Installation'
  - ln -s {{.workspace}}/opt/sonar-scanner-{{.sonarVersion}}-linux {{.workspace}}/opt/sonar
  - export PATH="${PATH}:{{.workspace}}/opt/sonar/bin"
  - ""
  - '# Runtime'
  - export SONAR_SCANNER_OPTS="-Xmx1024m"
  - cd {{.workspace}}
  - cat <<EOF > sonar-project.properties
  - '{{.sonar-project.properties}}'
  - EOF
  - ""
  - sonar-scanner -Dsonar.host.url={{.sonarURL}} -Dsonar.login={{.sonarUsername}}
    -Dsonar.password={{.sonarPassword}} -Dsonar.branch={{.sonarBranch}} -Dsonar.scm.disabled=true
```


Import a worker model:

```bash
cdsctl worker model import ./cds-docker-package.yml
```

Or with a remote file:

```bash
cdsctl action import https://raw.githubusercontent.com/ovh/cds/{{< param "version" "master" >}}/contrib/actions/cds-docker-package.yml
```
