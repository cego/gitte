# git-local-devop

[![quality](https://img.shields.io/github/workflow/status/firecow/git-local-devops/Quality)](https://github.com/firecow/git-local-devops/actions)
[![license](https://img.shields.io/github/license/firecow/git-local-devops)](https://npmjs.org/package/git-local-devops)
[![Renovate](https://img.shields.io/badge/renovate-enabled-brightgreen.svg)](https://renovatebot.com)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=firecow_git-local-devops&metric=alert_status)](https://sonarcloud.io/dashboard?id=firecow_git-local-devops)
[![Coverage](https://sonarcloud.io/api/project_badges/measure?project=firecow_git-local-devops&metric=coverage)](https://sonarcloud.io/dashboard?id=firecow_git-local-devops)
[![Code Smells](https://sonarcloud.io/api/project_badges/measure?project=firecow_git-local-devops&metric=code_smells)](https://sonarcloud.io/dashboard?id=firecow_git-local-devops)

Put `config.yml` in `~/git-local-devops`

```
---
startup:
  # Used to check host machine for 
  - { argv: ["git", "--version"], message: "Git isn't installed on the system" }
  - { argv: ["docker", "--version"], message: "Docker isn't installed on the system" }
  - { argv: ["docker", "login", "registry.gitlab.com"], message: "You must be logged in on registry.gitlab.com to fetch docker images" }

projects:
  - remote: git@gitlab.com:firecow/example.git
    default_branch: main
    scripts:
      up:
        firecow.dk: ["bash", "-c", "start-docker-stack.sh"]
        firecow.net: ["docker-compose", "up"]
      down:
        firecow.dk: ["docker", "stack", "rm", "firecow.dk"]
        firecow.net: ["docker-compose", "down"]
```

Run `git-local-devops up firecow.dk` inside `~/git-local-devops` folder

All projects specified will pull the latest changes if on default branch

All projects on custom branch, will attempt to rebase `origin/<default_branch>` first, if that fails a merge with `origin/<default_branch>` will be attempted.

After git operations are done, scripts matching this program inputs will be executed.

In this example only `start-docker-stack.sh` will be executed in `~/git-local-devops/firecow/example` checkout

