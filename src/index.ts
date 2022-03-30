import { runAction } from "./actions";
import { gitOperations } from "./git_operations";
import { startup } from "./startup";
import { getPriorityRange } from "./priority";
import { loadConfig } from "./config_loader";
import { printLogs } from "./utils";
import { searchOutputForHints } from "./search_output";
import { GroupKey } from "./types/utils";
import { Output } from "promisify-child-process";

export async function start(
	cwd: string,
	actionToRun: string,
	groupToRun: string,
): Promise<void> {
	const cnf = await loadConfig(cwd);

	await startup(Object.values(cnf.startup));

	const gitOperationsPromises = [];
	for (const projectObj of Object.values(cnf.projects)) {
		gitOperationsPromises.push(gitOperations(cwd, projectObj));
	}
	const logs = await Promise.all(
		gitOperationsPromises.map((p) => p.catch((e) => e)),
	);
	printLogs(Object.keys(cnf.projects), logs);

	const prioRange = getPriorityRange(Object.values(cnf.projects));

	const stdoutBuffer: (GroupKey & Output)[] = [];
	for (let i = prioRange.min; i < prioRange.max; i++) {
		const runActionPromises = [];
		for (const projectKey of Object.keys(cnf.projects)) {
			runActionPromises.push(
				runAction(
					cwd,
					cnf,
					{ project: projectKey, action: actionToRun, group: groupToRun },
					i,
				),
			);
		}
		const stdoutBufferPromises = await Promise.all(runActionPromises);
		stdoutBufferPromises
			.filter((p) => p)
			.forEach((p) => stdoutBuffer.push(p as GroupKey & { stdout: string }));
	}

	if (cnf.searchFor) searchOutputForHints(cnf.searchFor, stdoutBuffer);
}
