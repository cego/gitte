import { loadConfig } from "./config_loader";
import { TaskPlanner } from "./task_running/task_planner";

export async function getActionNames(argv: any): Promise<string[]> {
    const cnf = await loadConfig(argv.cwd, argv.needs);

    const taskPlanner = new TaskPlanner(cnf);
    const projects = taskPlanner.findProjects(['*']);

    return (new TaskPlanner(cnf)).findActions(projects, ['*'])
        .map(a => a.action);
}

export async function getGroupNames(argv: any, actionsStr: string): Promise<string[]> {
    const cnf = await loadConfig(argv.cwd, argv.needs);

    const actions: string[] = actionsStr.split('+');

    const taskPlanner = new TaskPlanner(cnf);
    const projects = taskPlanner.findProjects(['*']);
    const projectActions = taskPlanner.findActions(projects, actions);

    return taskPlanner.findGroups(projectActions, ['*'])
        .map(a => a.group);
}

export async function getProjectNames(argv: any, actionsStr: string, groupsStr: string): Promise<string[]> {
    const cnf = await loadConfig(argv.cwd, argv.needs);

    const actions: string[] = actionsStr.split('+');
    const groups: string[] = groupsStr.split('+');

    // given actions and groups, find compatible projects
    const taskPlanner = new TaskPlanner(cnf);
    const projects = taskPlanner.findProjects(['*']);
    const projectActions = taskPlanner.findActions(projects, actions);
    const keys = taskPlanner.findGroups(projectActions, groups);

    return [...keys.reduce((carry, key) => {
        return carry.add(key.project);
    }, new Set<string>())];
}

export async function tabCompleteActions(_:string, argv: any): Promise<string[]> {
    const words = argv._.slice(2);
    // console.log(words);
    // predict action
    if (words.length == 1) {
        const actionNames = [...new Set(await getActionNames(argv))];
        return appendToMultiple(words[0], actionNames);
    }

    // predict group
    if (words.length == 2) {
        const groupNames = [...new Set(await getGroupNames(argv, words[0]))];
        return appendToMultiple(words[1], groupNames);
    }

    // predict project
    if (words.length == 3) {
        const projectNames = [...new Set(await getProjectNames(argv, words[0], words[1]))];
        return appendToMultiple(words[2], projectNames);
    }

    return []
}

function appendToMultiple(word: string, options: string[]) {
    console.log({word})
    const previousActions = word.split('+');
    if(previousActions.length == 1) {
        return options;
    }

    // remove last action
    previousActions.pop();
    const previousActionsString = previousActions.join('+');
    return options
        .filter(name =>
            !previousActions.includes(name)
        ).map(name => {
            return previousActionsString + '+' + name;
        })
}

export function tabCompleteClean(argv: any) {
    // slice first word off
    const words = argv._.slice(2);
    
    if(words.length == 1) {
        return [
            "untracked", "local-changes", "master", "non-gitte", "all"
        ];
    }

    return [];
}

export async function tabCompleteToggle(argv: any) {
    // slice first word off
    const words = argv._.slice(2);

    if(words.length == 1) {
        const cnf = await loadConfig(argv.cwd, argv.needs);
        const taskPlanner = new TaskPlanner(cnf);
        return taskPlanner.findProjects(['*']);
    }

    return [];
}
