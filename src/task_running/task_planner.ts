import { getProjectDirFromRemote } from "../project";
import { Config } from "../types/config";
import { GroupKey } from "../types/utils";
import { Task } from "./task";

type ProjectAction = {
    project: string;
    action: string;
}

/**
 * Given an input of 
 * - Actions
 * - Groups
 * - Projects
 * 
 * Generate the task list needed to run.
 */
class TaskPlanner {
    constructor(
        private config: Config,
    ) { }

    public planStringInput(
        actionsStr: string,
        groupsStr: string,
        projectsStr: string,
    ) {
        let actions = actionsStr.split("+");
        let groups = groupsStr.split("+");
        let projects = projectsStr.split("+");

        // rewrite "all" to "*"
        actions = actions.map(action => { return action == "all" ? "*" : action; });
        groups = groups.map(group => { return group == "all" ? "*" : group; });
        projects = projects.map(project => { return project == "all" ? "*" : project; });

        return this.plan(actions, groups, projects);
    }

    public plan(
        actions: string[],
        groups: string[],
        projects: string[],
    ): Task[] {
        // First find all actions explicitly.
        const keySets = this.findKeySets(actions, groups, projects);

        // For each key set, create a task.
        return keySets.map(keySet => {
            const project = this.config.projects[keySet.project];
            const action = project.actions[keySet.action];
            const needs = action.needs.map((need: string) => ({ ...keySet, action: need}));
            const cwd = getProjectDirFromRemote(this.config.cwd, project.remote);

            return new Task(keySet, { cwd, cmd: action.groups[keySet.group], priority: action.priority ?? 0}, needs)
        });
    }

    findKeySets(actionsStr: string[], groupsStr: string[], projectsStr: string[]): GroupKey[] {
        const projects = this.findProjects(projectsStr);
        const actions = this.findActions(projects, actionsStr);
        const keySets = this.findGroups(actions, groupsStr);

        if(this.config.needs) {
            // Resolve dependencies between projects of same action and group.
            return this.addProjectDependencies(keySets);
        }
        // todo i think we need to remove dependencies from needs, even if we dont intend to run them.
        return keySets;
    }

    addProjectDependencies(keySets: GroupKey[]): GroupKey[] {
        // go through each key set, and verify that all dependencies are met.
        for(const keySet of keySets) {
            const project = this.config.projects[keySet.project];
            const action = project.actions[keySet.action];
            const needs = action.needs.map((need: string) => ({ ...keySet, action: need}));
            const missing = needs.filter(need => !keySets.includes(need));
            
            for(const missingKeySet of missing) {
                const missingAction = project.actions[missingKeySet.action];
                // if the group 
                if(this.config.projects[missingKeySet.project].actions[missingKeySet.action].groups[missingKeySet.group]) {
                    keySets.push(missingKeySet);
                }
                else {

                }

            
        }
        throw new Error("Method not implemented.");
    }

    findProjects(projectsStr: string[]): string[] {
        if (projectsStr.includes("*")) {
            return Object.keys(this.config.projects)
        }

        return Object.keys(this.config.projects)
            .filter((projectName) => projectsStr.includes(projectName))
    }

    findActions(projects: string[], actionsStr: string[]): ProjectAction[] {
        // Collect all matching actions from all projects
        return projects.reduce(
            (carry, projectName) => {
                const project = this.config.projects[projectName];
                const actions = Object.keys(project.actions)
                    .filter((actionName) => actionsStr.includes(actionName) || actionsStr.includes("*"))
                    .map((actionName) => ({ project: projectName, action: actionName }));
                return [...carry, ...actions];
            }, [] as ProjectAction[]);
    }

    findGroups(projectActions: ProjectAction[], groupsStr: string[]): GroupKey[] {
        return projectActions.reduce(
            (carry, projectAction) => {
                const action = this.config.projects[projectAction.project].actions[projectAction.action];
                const groups = Object.keys(action.groups).filter(
                    (groupName) => groupsStr.includes(groupName) || groupsStr.includes("*")
                ).map((groupName) => ({ ...projectAction, group: groupName }));
                return [...carry, ...groups];
            }, [] as GroupKey[]);
    }
}