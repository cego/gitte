import { loadConfig } from "../../src/config_loader";
import { startup } from "../../src/startup";
import { fromConfig as gitOpsFromConfig } from "../../src/gitops";
import { Argv } from "yargs";
import { errorHandler } from "../../src/error_handler";
import { actionsBuilder } from "./actions";
import { TaskHandler } from "../../src/task_running/task_handler";

// noinspection JSUnusedGlobalSymbols
export function builder(y: Argv) {
	return actionsBuilder(y);
}
// noinspection JSUnusedGlobalSymbols
export const command = "run <action> <group> [projects]";
// noinspection JSUnusedGlobalSymbols
export const describe = "Run startup, git operations and actions on all projects";
// noinspection JSUnusedGlobalSymbols
export async function handler(argv: any) {
	try {
		const cnf = await loadConfig(argv.cwd, argv.needs);
		await startup(cnf);
		await gitOpsFromConfig(cnf, argv.autoMerge);
		await new TaskHandler(cnf, argv.action, argv.group, argv.projects, argv.maxTaskParallelization).run();
	} catch (e) {
		errorHandler(e);
	}
}
