import { loadConfig } from "./config_loader";
import { TaskPlanner } from "./task_running/task_planner";

export async function getActionNames(argv: any):Promise<string[]> {
    const cnf = await loadConfig(argv.cwd, argv.needs);

    return (new TaskPlanner(cnf)).findActions(['*'], ['*'])
        .map(a => a.action);
}

export async function getGroupNames(argv: any):Promise<string[]> {
    const cnf = await loadConfig(argv.cwd, argv.needs);

    const actions: string[] = argv.actions.split('+');

    const taskPlanner = new TaskPlanner(cnf);
    const projectActions = taskPlanner.findActions(actions, ['*']);

    return taskPlanner.findGroups(projectActions, ['*'])
        .map(a => a.group);
}

export async function getProjectNames(argv: any):Promise<string[]> {
    const cnf = await loadConfig(argv.cwd, argv.needs);

    const actions: string[] = argv.actions.split('+');
    const groups: string[] = argv.groups.split('+');

    // given actions and groups, find compatible projects
    const taskPlanner = new TaskPlanner(cnf);
    const projectActions = taskPlanner.findActions(actions, ['*']);
    const keys = taskPlanner.findGroups(projectActions, ['*'])
        .filter(a => groups.includes(a.group));

    return [...keys.reduce((carry, key) => {
        return carry.add(key.project);
    }, new Set<string>())];
}