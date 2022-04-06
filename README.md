# gitte

[![quality](https://img.shields.io/github/workflow/status/cego/gitte/Quality)](https://github.com/cego/gitte/actions)
[![license](https://img.shields.io/github/license/cego/gitte)](https://npmjs.org/package/gitte)
[![Renovate](https://img.shields.io/badge/renovate-enabled-brightgreen.svg)](https://renovatebot.com)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=cego_gitte&metric=alert_status)](https://sonarcloud.io/dashboard?id=cego_gitte)
[![Coverage](https://sonarcloud.io/api/project_badges/measure?project=cego_gitte&metric=coverage)](https://sonarcloud.io/dashboard?id=cego_gitte)
[![Code Smells](https://sonarcloud.io/api/project_badges/measure?project=cego_gitte&metric=code_smells)](https://sonarcloud.io/dashboard?id=cego_gitte)

## Installation

### Linux

```bash
curl -s "https://cego.github.io/gitte/ppa/pubkey.gpg" | sudo apt-key add -
sudo curl -s -o /etc/apt/sources.list.d/gitte.list "https://cego.github.io/gitte/ppa/gitte.list"
sudo apt-get update
sudo apt-get install gitte
```

## Config setup

Put `.gitte.yml` in a designated folder. For reference on what should be in this config, see [config documentation](./docs/config.md)

You can also use a remote config file if a file exists named `.gitte-env`.

```
REMOTE_GIT_REPO="git@gitlab.com:cego/example.git"
REMOTE_GIT_FILE=".gitte.yml"
REMOTE_GIT_REF="master"
```

gitte will search your current folder for a config. A env file will have higher priority than `.yml`. If gitte fail to find a configuration in your current folder, it will try parent folders.

## Usage

`gitte run <action> <group>`

All startup checks will run. If they pass, all projects will be updated and the desired action and group will be run for each project.

### Git operations

`gitte gitops`

All projects will pull the latest changes if there are no local changes.

An optional option `--auto-merge` can be supplied, that will automatically merge origin/<default_branch> into each project. This can also be set by the env variable `GITTE_AUTO_MERGE=true`.

### Running actions without startup checks and gitops

`gitte actions up cego.dk`

All projects will run the action `up` with the group `cego.dk` in this case.

### Running startup checks alone

`gitte startup`

### Listing all projects and their actions

`gitte list`

### Validate the config and dependency graph

`gitte validate`

### Other

For more information on other options, try running

`gitte --help`
