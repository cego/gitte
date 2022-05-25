# gitte

[![quality](https://img.shields.io/github/workflow/status/cego/gitte/Quality)](https://github.com/cego/gitte/actions)
[![license](https://img.shields.io/github/license/cego/gitte)](https://npmjs.org/package/gitte)
[![Renovate](https://img.shields.io/badge/renovate-enabled-brightgreen.svg)](https://renovatebot.com)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=cego_gitte&metric=alert_status)](https://sonarcloud.io/dashboard?id=cego_gitte)
[![Coverage](https://sonarcloud.io/api/project_badges/measure?project=cego_gitte&metric=coverage)](https://sonarcloud.io/dashboard?id=cego_gitte)
[![Code Smells](https://sonarcloud.io/api/project_badges/measure?project=cego_gitte&metric=code_smells)](https://sonarcloud.io/dashboard?id=cego_gitte)

Tool to help keep a range of projects up to date with git version control, and also help execute commands and scripts across projects. For configuration options please refer to [config documentation](./docs/config.md).

# Installation

## Install using npm

Requires npm and node version 16 or higher.

```
npm install -g @cego/gitte
```

## Linux binaries

```bash
curl -s "https://cego.github.io/gitte/ppa/pubkey.gpg" | sudo apt-key add -
sudo curl -s -o /etc/apt/sources.list.d/gitte.list "https://cego.github.io/gitte/ppa/gitte.list"
sudo apt-get update
sudo apt-get install gitte
```

# Basic usage

In a terminal in a folder with a gitte configuration, or a subfolder thereof, run:

```
$ gitte run <actions> <groups> [projects]`
```

Gitte will then do the following

1. Run all specified startup checks. If any fail, exit.
2. Try to update all projects with git pull. Will inform the user if update is not possible. Gitte should never overwrite local changes.
3. Execute the desired action with the given group. The optional project parameter can be used to limit the projects the action and group will run in.

> An optional option `--auto-merge` can be supplied, that will automatically merge origin/<default_branch> into each project, if you are on a non-default branch without local changes or conflicts. This can also be set by the env variable `GITTE_AUTO_MERGE=true`.

## Wildcards and lists

All three parameters support the wildcard '\*' which will run all action, groups or projects. For example one might want to run all actions in all groups, which can be accomplished with

```
gitte run '*' '*'
```

If you want to specify multiple actions, groups or project, please use the `+` operator.

```
gitte run build+deploy example.com
```

## Other commands

For other usage, such as running startup, git operations or actions seperately, please refer to [commands documentation](./docs/commands.md), or see `gitte --help`.

## Override and exclude projects

If the file `.gitte-override.yml` exist in the same folder as `.gitte.yml` or `.gitte-env` it will automatically be merged.

If the file `.gitte-projects-disable` exist, projects, seperated by a newline, will be excluded from gitte.
