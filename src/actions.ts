import { getProjectDirFromRemote } from "./project";
import chalk from "chalk";
import { default as to } from "await-to-js";
import { Project } from "./types/config";
import * as pcp from "promisify-child-process";
import { ToChildProcessOutput } from "./types/utils";

export async function runActions(cwd: string, project: Project, currentPrio: number, actionToRun: string, groupToRun: string): Promise<{ [key: string]: string[] }> {
	const remote = project.remote;
	const dir = getProjectDirFromRemote(cwd, remote);

	const stdoutHistory: { [key: string]: string[] } = {};

	for (const [actionName, actionObj] of Object.entries(project.actions)) {
		if (actionName !== actionToRun) continue;

		const stdoutBuffer: string[] = [];
		for (const [groupName, cmd] of Object.entries(actionObj.groups)) {
			const priority = actionObj["priority"] ?? project["priority"] ?? 0;
			if (currentPrio !== priority) continue;
			if (groupName !== groupToRun) continue;
			console.log(chalk`{blue ${cmd.join(" ")}} is running in {cyan ${dir}}`);
			const [err, res]: ToChildProcessOutput = await to(pcp.spawn(cmd[0], cmd.slice(1), { cwd: dir, env: process.env, encoding: "utf8" }));
			if (err) {
				console.error(chalk`"${actionToRun}" "${groupToRun}" {red failed}, goto {cyan ${dir}} and run {blue ${cmd.join(" ")}} manually`);
			}
			if (res?.stdout) stdoutBuffer.push(res.stdout.toString());
		}

		if (stdoutBuffer.length > 0) {
			stdoutHistory[actionObj.groups[groupToRun].join(" ")] = stdoutBuffer;
		}
	}
	return stdoutHistory;
}
