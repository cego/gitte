import { runActions } from "./actions";
import { gitOperations } from "./git_operations";
import { startup } from "./startup";
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

	const gitOperationsPromises = Object.values(cnf.projects).map((project) =>
		gitOperations(cwd, project),
	);
	const logs = await Promise.all(
		gitOperationsPromises.map((p) => p.catch((e) => e)),
	);
	printLogs(Object.keys(cnf.projects), logs);

	const stdoutBuffer: (GroupKey & Output)[] = await runActions(
		cnf,
		cwd,
		actionToRun,
		groupToRun,
	);
	if (cnf.searchFor) searchOutputForHints(cnf.searchFor, stdoutBuffer);
}
