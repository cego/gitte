# git-local-devop

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

All projects specified will pull latest changes if on default branch

All projects on custom branch, will attempt to rebase origin/<default_branch> first, if that fails a merge with origin/<default_branch> will be attempted.

After git operations are done, scripts matching this program inputs will be executed.

In this example only `start-docker-stack.sh` will be executed in `~/git-local-devops/firecow/example` checkout

