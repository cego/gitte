import {getProjectDirFromRemote} from "./project";
import chalk from "chalk";
import {default as to} from "await-to-js";
import { Project } from "./validate_yaml";
import { asyncSpawn } from "./utils";

export async function runActions(cwd: string, project: Project, currentPrio: number, actionToRun: string, groupToRun: string) {
	const remote = project.remote;
	const dir = getProjectDirFromRemote(cwd, remote);
	let err;

	for (const [actionName, actionObj] of Object.entries(project.actions)) {
		if (actionName !== actionToRun) continue;

		for (const [groupName, cmd] of Object.entries(actionObj.groups)) {
			const priority = actionObj["priority"] ?? project["priority"] ?? 0;
			if (currentPrio !== priority) continue;
			if (groupName !== groupToRun) continue;
			console.log(chalk`{blue ${cmd.join(" ")}} is running in {cyan ${dir}}`);
			[err] = await to(asyncSpawn(cmd[0], cmd.splice(1), {cwd: dir, env: process.env}));
			if (err) {
				console.error(chalk`"${actionToRun}" "${groupToRun}" {red failed}, goto {cyan ${dir}} and run {blue ${cmd.join(" ")}} manually`);
			}
		}
	}
}