# gitte

[![quality](https://img.shields.io/github/actions/workflow/status/cego/gitte/quality.yml?branch=main)](https://github.com/cego/gitte/actions)
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

### Linux based on Debian

Users of Debian-based distributions should prefer the [the Deb822 format][deb822], installed with:

```bash
sudo wget -O /etc/apt/sources.list.d/gitte.sources https://gitte.cego.dk/gitte.sources
sudo apt-get update
sudo apt-get install gitte
```

[deb822]: https://repolib.readthedocs.io/en/latest/deb822-format.html#deb822-format

If your distribution does not support this, you can run these commands:

```bash
curl -s "https://gitte-ppa.cego.dk/pubkey.gpg" | sudo apt-key add -
echo "deb https://gitte-ppa.cego.dk ./" | sudo tee /etc/apt/sources.list.d/gitte.list
sudo apt-get update
sudo apt-get install gitte
```

Note that the path `/etc/apt/sources.list.d/gitte.list` is used in the file `gitte.list`.
If you change it in these commands you must also change it in `/etc/apt/sources.list.d/gitte.list`.

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

If configured, gitte is able to switch automatically between groups. Switching between groups involve downing all other groups than specified, then upping the specified group.

```
$ gitte switch <group>
```

## Wildcards and lists

All three parameters support the wildcard '\*' which will run all action, groups or projects. For example one might want to run all actions in all groups, which can be accomplished with

```
gitte run '*' '*'
```

If you want to specify multiple actions, groups or project, please use the `+` operator.

```
gitte run build+deploy example.com
```

## Disabling projects

It is possible to disable projects completely. This can be done using the `toggle` command.

To see a current list of enabled/disabled projects:

`gitte toggle`

To toggle a project:

`gitte toggle <project>`

To reset to default state:

`gitte toggle reset`

## Other commands

For other usage, such as running startup, git operations or actions seperately, please refer to [commands documentation](./docs/commands.md), or see `gitte --help`.

## Override and exclude projects

If the file `.gitte-override.yml` exist in the same folder as `.gitte.yml` or `.gitte-env` it will automatically be merged.

If the file `.gitte-projects-disable` exist, projects, seperated by a newline, will be excluded from gitte.

## Environment variables

### GITTE_AUTO_MERGE

Default: false

Gitte will automatically merge default branch into custom branches if this is set to true.

### GITTE_CWD

Default: cwd of the current process

Gitte will use this as the current working directory.

### GITTE_NO_NEEDS

Default: false (false = needs are enabled)

Ignore dependencies.

### GITTE_MAX_TASK_PARALLELIZATION

Default: CPU/2

Set this to limit the number of parallel processes when running tasks.

## How to publish debian packages to [gitte-ppa.cego.dk](gitte-ppa.cego.dk)

Run `./publish-os-packages` and upload the ppa/ppa.zip file to cego's cloudflare pages

A gpg signing key is needed to sign the debian packages.

## How to publish npmjs.com

Run `npm publish` to upload to npmjs.com

You need proper permissions in the `@cego` organization on [npmjs.com](npmjs.com)
