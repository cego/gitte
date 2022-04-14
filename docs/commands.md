## Other commands

Besides the main `run` command, other commands can be used in certain scenarios.

### Git operations

```
$ gitte gitops
```

All projects will pull the latest changes if there are no local changes.

### Running actions without startup checks and gitops

```
$ gitte actions up cego.dk
```

All projects will run the action `up` with the group `cego.dk` in this case. Arguments can be specified as shown in

### Running startup checks alone

```
$ gitte startup
```

### Listing all projects and their actions

```
$ gitte list
```

### Validate the config and dependency graph

```
$ gitte validate
```

### Other

For more information on other options, try running

```
$ gitte --help
```
