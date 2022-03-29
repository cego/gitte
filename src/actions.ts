import { getProjectDirFromRemote } from "./project";
import chalk from "chalk";
import { default as to } from "await-to-js";
import { Config, Project } from "./types/config";
import * as pcp from "promisify-child-process";
import { getPriorityRange } from "./priority";

interface ActionsOpt {
	cwd: string;
	project: Project;
	currentPrio: number;
	actionToRun: string;
	groupToRun: string;
}

export async function actions(opt: ActionsOpt) {
	const remote = opt.project.remote;
	const dir = getProjectDirFromRemote(opt.cwd, remote);
	let err;

	for (const [actionName, actionObj] of Object.entries(opt.project.actions)) {
		if (actionName !== opt.actionToRun) continue;

		for (const [groupName, cmd] of Object.entries(actionObj.groups)) {
			const priority = actionObj["priority"] ?? opt.project["priority"] ?? 0;
			if (opt.currentPrio !== priority) continue;
			if (groupName !== opt.groupToRun) continue;
			console.log(chalk`{blue ${cmd.join(" ")}} is running in {cyan ${dir}}`);
			[err] = await to(pcp.spawn(cmd[0], cmd.slice(1), { cwd: dir, env: process.env }));
			if (err) {
				console.error(
					chalk`"${opt.actionToRun}" "${opt.groupToRun}" {red failed}, goto {cyan ${dir}} and run {blue ${cmd.join(
						" ",
					)}} manually`,
				);
			}
		}
	}
}

export async function fromConfig(cwd: string, cnf: Config, actionToRun: string, groupToRun: string) {
	const prioRange = getPriorityRange(Object.values(cnf.projects));
	for (let i = prioRange.min; i < prioRange.max; i++) {
		const runActionPromises = [];
		for (const projectObj of Object.values(cnf.projects)) {
			runActionPromises.push(actions({ cwd, project: projectObj, currentPrio: i, actionToRun, groupToRun }));
		}
		await Promise.all(runActionPromises);
	}
}
