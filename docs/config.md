# Config setup

This document solely explains what configuration is available in the `.git-local-devops.yml` file.

## Startup checks

Startup checks is a list of commands or shell scripts that will be run before anything else when the `run <action> <group>` command is ran.

If the exit code of a startup script is not 0, the run will stop, and print the supplied hint.

A startup-check must supply a hint, and either a cmd or shell property like in below example:

```yaml
---
startup:
  # Used to check host machine for various requirements.
  git-present: { cmd: ["git", "--version"], hint: "Git isn't installed on the system" }
  docker-present: { cmd: ["docker", "--version"], hint: "Docker isn't installed on the system" }
  docker-login:
    {
      cmd: ["docker", "login", "registry.gitlab.com"],
      hint: "You must be logged in on registry.gitlab.com to fetch docker images",
    }
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
```

## Projects

Projects define what projects should be kept up to date and cloned, and also what actions are available on each project.

A project must contain the properties remote and default_branch.

A project can contain many actions.

```yaml
---
projects:
  example:
    remote: git@gitlab.com:cego/example.git
    default_branch: main
    actions:
      up:
        needs: [anotherproject, andathirdproject]
        groups:
          cego.dk: ["bash", "-c", "start-docker-stack.sh"]
          cego.net: ["docker-compose", "up"]
      down:
        priority: 1
        groups:
          cego.dk: ["docker", "stack", "rm", "cego.dk"]
          cego.net: ["docker-compose", "down"]
```

### Actions

An action on a project must specify the group property. The group property is a mapping from a group name, to a command. If you want an action for a project to run on any group, you can use the wildcard `*`.

Besides a group property, actions can contain a priority or a needs dependency. A higher priority (lower nummber) will always be executed before a lower priority. Like wise, if a needs b, then b will always be exucted before a.

Please note it is not allowed for an action to have both a priority and a needs dependency. It is also required that the dependency graph is acyclic.

Besides these properties, an action can also contain a searchFor property

## Search for

After an action is run, its output can be searched for.

To search all actions, you can add a root property named `searchFor`. To only search some actions, you can add a `searchFor` property to each action as you desire.

A searchFor object consists of a regex and a hint. The regex main contain groups, and the hint can print matched groups. Searchfor also supports chalk-templates. See examples below.

```yaml
searchFor:
  - regex: "Error: Timeout exceeded\\n.*Deployment to <[^|]*\\|([^>]*)> \\*FAILED\\* in \\d*s"
    hint: "{bgYellow  WARN } {1} failed due to timeout. Did you remember to run build? {cyan git-local-devops run build <site>}"
  - regex: "\\n[^\"]*Visit https:\\/\\/registry\\.gitlab\\.com\\/ to find login information"
    hint: "{bgYellow  WARN } Login check to registry.gitlab.com failed"
```
