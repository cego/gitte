# git-local-devops

[![quality](https://img.shields.io/github/workflow/status/cego/git-local-devops/Quality)](https://github.com/cego/git-local-devops/actions)
[![license](https://img.shields.io/github/license/cego/git-local-devops)](https://npmjs.org/package/git-local-devops)
[![Renovate](https://img.shields.io/badge/renovate-enabled-brightgreen.svg)](https://renovatebot.com)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=cego_git-local-devops&metric=alert_status)](https://sonarcloud.io/dashboard?id=cego_git-local-devops)
[![Coverage](https://sonarcloud.io/api/project_badges/measure?project=cego_git-local-devops&metric=coverage)](https://sonarcloud.io/dashboard?id=cego_git-local-devops)
[![Code Smells](https://sonarcloud.io/api/project_badges/measure?project=cego_git-local-devops&metric=code_smells)](https://sonarcloud.io/dashboard?id=cego_git-local-devops)

## Installation
On most Linux distributions, install using apt:

`sudo apt install git-local-devops`

On Mac systems, install with brew:

`brew install git-local-devops`

At this time there is no supplied Windows binaries.

## Config setup

Put `.git-local-devops.yml` in a designated folder. For reference on what should be in this config, see [config documentation](./docs/config.md)

You can also use a remote config file if a file exists named `.git-local-devops-env`.

```
REMOTE_GIT_REPO="git@gitlab.com:cego/example.git"
REMOTE_GIT_FILE=".git-local-devops.yml"
REMOTE_GIT_REF="master"
```

Git-local-devops will search your current folder for a config. A env file will have higher priority than `.yml`. If git-local-devops fail to find a configuration in your current folder, it will try parent folders.

## Usage

`git-local-devops run <action> <group>`

All startup checks will run. If they pass, all projects will be updated and the desired action and group will be run for each project.

### Git operations

`git-local-devops gitops`

All projects will pull the latest changes and/or merge with origin/<default_branch>

### Running actions without startup checks and gitops

`git-local-devops actions up cego.dk`

All projects will run the action `up` with the group `cego.dk` in this case.

### Running startup checks alone

`git-local-devops startup`

### Listing all projects and their actions

`git-local-devops list`

### Validate the config and dependency graph

`git-local-devops validate`

### Other

For more information on other options, try running

`git-local-devops --help`
