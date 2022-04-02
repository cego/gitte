# git-local-devops

[![quality](https://img.shields.io/github/workflow/status/cego/git-local-devops/Quality)](https://github.com/cego/git-local-devops/actions)
[![license](https://img.shields.io/github/license/cego/git-local-devops)](https://npmjs.org/package/git-local-devops)
[![Renovate](https://img.shields.io/badge/renovate-enabled-brightgreen.svg)](https://renovatebot.com)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=cego_git-local-devops&metric=alert_status)](https://sonarcloud.io/dashboard?id=cego_git-local-devops)
[![Coverage](https://sonarcloud.io/api/project_badges/measure?project=cego_git-local-devops&metric=coverage)](https://sonarcloud.io/dashboard?id=cego_git-local-devops)
[![Code Smells](https://sonarcloud.io/api/project_badges/measure?project=cego_git-local-devops&metric=code_smells)](https://sonarcloud.io/dashboard?id=cego_git-local-devops)

## Config setup

Put `.git-local-devops.yml` in `~/git-local-devops` or another user owned folder.

```
---
startup:
  # Used to check host machine for various requirements.
  git-present:
    { cmd: ["git", "--version"], hint: "Git isn't installed on the system" }
  docker-present:
    { cmd: ["docker", "--version"], hint: "Docker isn't installed on the system" }
  docker-login:
    { cmd: ["docker", "login", "registry.gitlab.com"], hint: "You must be logged in on registry.gitlab.com to fetch docker images" }
  ensure-docker-swarm-networks:
    shell: "bash"
    script: |
      docker_overlay_networks="swarm-network"
      for docker_overlay_network in ${docker_overlay_networks}
      do
        if (docker network ls | grep -w " ${docker_overlay_network} " 1> /dev/null)
        then
          echo "${docker_overlay_network} network exists, doing nothing"
        else
          echo "Creating ${docker_overlay_network} network"
          docker network create "${docker_overlay_network}" --driver overlay --opt encrypted --attachable 1> /dev/null
        fi
      done


projects:
  example:
    remote: git@gitlab.com:cego/example.git
    default_branch: main
    actions:
      up:
        groups:
            cego.dk: ["bash", "-c", "start-docker-stack.sh"]
            cego.net: ["docker-compose", "up"]
      down:
        priority: 1
        groups:
            cego.dk: ["docker", "stack", "rm", "cego.dk"]
            cego.net: ["docker-compose", "down"]
```

You can also use a remote config file if you put `.git-local-devops-env` in `~/git-local-devops`

```
REMOTE_GIT_REPO="git@gitlab.com:cego/example.git"
REMOTE_GIT_FILE=".git-local-devops.yml"
REMOTE_GIT_REF="master"
```

## Running scripts

Run `git-local-devops up cego.dk` inside `~/git-local-devops` folder

All projects specified will pull the latest changes if on default branch

All projects on custom branch, will attempt to rebase `origin/<default_branch>` first, if that fails a merge with `origin/<default_branch>` will be attempted.

After git operations are done, scripts matching cli inputs will be executed.

In this example only `"bash", "-c", "start-docker-stack.sh"` will be executed in `~/git-local-devops/cego/example` checkout

## Execution order
You may specify either a priority or a needs array for each action, but never both.
The needs array must point to other project names and must be acyclic.

If there is no priority or needs, the action has a default priority of 0.

Execution order is as follows:
1. Execute the lowest priority actions (Will execute in parallel if same priority)
2. When these actions finish, remove their needs from other action that needs these actions
    - If this result in an action with an empty needs array, it will start execution of that action, then go back to step 2.
3. Remove the lowest priority actions and go back to step 1.
