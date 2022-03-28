
import { runActions } from "./actions";
import { gitOperations } from "./git_operations";
import { startup } from "./startup";
import { getPriorityRange } from "./priority";
import { loadConfig } from "./config_loader";
import { printLogs } from "./utils";
import { searchStdoutAndPrintHints } from "./search_stdout";

export async function start(cwd: string, actionToRun: string, groupToRun: string): Promise<void> {
	const cnf = await loadConfig(cwd);

	await startup(Object.values(cnf.startup));

	const gitOperationsPromises = [];
	for (const projectObj of Object.values(cnf.projects)) {
		gitOperationsPromises.push(gitOperations(cwd, projectObj));
	}
	const logs = await Promise.all(gitOperationsPromises.map(p => p.catch(e => e)));
	printLogs(Object.keys(cnf.projects), logs);

	const prioRange = getPriorityRange(Object.values(cnf.projects));

	let stdoutBuffer: { [key: string]: string[] } = {};
	for (let i = prioRange.min; i < prioRange.max; i++) {
		const runActionPromises = [];
		for (const projectObj of Object.values(cnf.projects)) {
			runActionPromises.push(runActions(cwd, projectObj, i, actionToRun, groupToRun));
		}
		stdoutBuffer = { ...stdoutBuffer, ...(await Promise.all(runActionPromises)).reduce((acc, cur) => ({ ...acc, ...cur }), {}) };
	}

	if(cnf.searchFor) searchStdoutAndPrintHints(cnf.searchFor,stdoutBuffer);
}

