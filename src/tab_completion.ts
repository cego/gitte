import { TaskPlanner } from "./task_running/task_planner";
import { loadCacheCwd } from "./cache";
import { Config } from "./types/config";

export function getActionNames(config: Config): string[] {
	const taskPlanner = new TaskPlanner(config);
	const projects = taskPlanner.findProjects(["*"]);

	return taskPlanner.findActions(projects, ["*"]).map((a) => a.action);
}

const rewriteAllToStar = (name: string) => {
	return name === "all" ? "*" : name;
};

export function getGroupNames(config: Config, actionsStr: string): string[] {
	const actions: string[] = actionsStr.split("+").map(rewriteAllToStar);

	const taskPlanner = new TaskPlanner(config);
	const projects = taskPlanner.findProjects(["*"]);
	const projectActions = taskPlanner.findActions(projects, actions);

	return taskPlanner.findGroups(projectActions, ["*"]).map((a) => a.group);
}

export function getProjectNames(config: Config, actionsStr: string, groupsStr: string): string[] {
	const actions: string[] = actionsStr.split("+").map(rewriteAllToStar);
	const groups: string[] = groupsStr.split("+").map(rewriteAllToStar);

	// given actions and groups, find compatible projects
	const taskPlanner = new TaskPlanner(config);
	const projects = taskPlanner.findProjects(["*"]);
	const projectActions = taskPlanner.findActions(projects, actions);
	const keys = taskPlanner.findGroups(projectActions, groups);

	return [
		...keys.reduce((carry, key) => {
			return carry.add(key.project);
		}, new Set<string>()),
	];
}

export function tabCompleteActions(_: string, argv: any): string[] {
	const cache = loadCacheCwd(argv.cwd);
	if (!cache) return [];

	const config = cache.config;
	const words = argv._.slice(2);

	// predict action
	if (words.length == 1) {
		const actionNames = [...new Set(getActionNames(config))];
		return appendToMultiple(words[0], actionNames);
	}

	// predict group
	if (words.length == 2) {
		const groupNames = [...new Set(getGroupNames(config, words[0]))];
		return appendToMultiple(words[1], groupNames);
	}

	// predict project
	if (words.length == 3) {
		const projectNames = [...new Set(getProjectNames(config, words[0], words[1]))];
		return appendToMultiple(words[2], projectNames);
	}

	return [];
}

function appendToMultiple(word: string, options: string[]) {
	// append all to options
	options = ["all", ...options];

	const previousActions = word.split("+");
	if (previousActions.length == 1) {
		return options;
	}

	// remove last action
	previousActions.pop();
	const previousActionsString = previousActions.join("+");
	return options
		.filter((name) => !previousActions.includes(name))
		.map((name) => {
			return previousActionsString + "+" + name;
		});
}

export function tabCompleteClean(argv: any) {
	// slice first word off
	const words = argv._.slice(2);

	if (words.length == 1) {
		return ["untracked", "local-changes", "master", "non-gitte", "all"];
	}

	return [];
}

export function tabCompleteToggle(argv: any): string[] {
	// slice first word off
	const words = argv._.slice(2);

	if (words.length == 1) {
		const cache = loadCacheCwd(argv.cwd);
		if (!cache) return [];

		const config = cache.config;
		const taskPlanner = new TaskPlanner(config);
		return taskPlanner.findProjects(["*"]);
	}

	return [];
}
