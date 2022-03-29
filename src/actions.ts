import { getProjectDirFromRemote } from "./project";
import chalk from "chalk";
import { default as to } from "await-to-js";
import { Config } from "./types/config";
import * as pcp from "promisify-child-process";
import { GroupKey, ToChildProcessOutput } from "./types/utils";

export async function runAction(
	cwd: string,
	config: Config,
	keys: GroupKey,
	currentPrio: number,
): Promise<(GroupKey & pcp.Output) | undefined> {
	if (!(keys.project in config.projects)) return;
	const project = config.projects[keys.project];

	const dir = getProjectDirFromRemote(cwd, project.remote);

	if (!(keys.action in project.actions)) return;
	const action = project.actions[keys.action];

	if (!(keys.group in action.groups)) return;
	const group = action.groups[keys.group];

	const priority = action.priority ?? project.priority ?? 0;

	if (currentPrio !== priority) return;

	console.log(chalk`{blue ${group.join(" ")}} is running in {cyan ${dir}}`);
	const [err, res]: ToChildProcessOutput = await to(
		pcp.spawn(group[0], group.slice(1), {
			cwd: dir,
			env: process.env,
			encoding: "utf8",
		}),
	);

	if (err) {
		console.error(
			chalk`"${keys.action}" "${
				keys.group
			}" {red failed}, goto {cyan ${dir}} and run {blue ${group.join(
				" ",
			)}} manually`,
		);
	}

	return {
		...keys,
		stdout: res?.stdout?.toString() ?? "",
		stderr: res?.stderr?.toString() ?? "",
	};
}
