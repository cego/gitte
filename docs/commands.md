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

### Cleanup projects

To clean up projects, use the `clean` command.

```
$ gitte clean
```

It can be run with 4 different options:

- `untracked` will remove all untracked files in all projects. (`git clean -fdx`)
- `local-changes` will remove local-changes in all projects. Will ask for confirmation before deleting anything.
- `master` will checkout the default branch in all projects.
- `non-gitte` will remove folders that gitte did not create. Will ask for confirmation before deleting anything.

If no option is specified, it will do all of the above.

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
