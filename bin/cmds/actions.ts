import { loadConfig } from "../../src/config_loader";
import { Argv } from "yargs";
import { errorHandler } from "../../src/error_handler";
import { TaskHandler } from "../../src/task_running/task_handler";
import { tabCompleteActions } from "../../src/tab_completion";
import os from "os";

// noinspection JSUnusedGlobalSymbols
export function builder(y: Argv) {
	return actionsBuilder(y);
}
// noinspection JSUnusedGlobalSymbols
export const command = "actions <actions> <groups> [projects]";
// noinspection JSUnusedGlobalSymbols
export const describe = "Run actions on selected projects for <actions> and <groups>";
// noinspection JSUnusedGlobalSymbols
export async function handler(argv: any) {
	try {
		const cnf = await loadConfig(argv.cwd, argv.needs);
		await TaskHandler.fromActionGroupProjectStrings(
			cnf,
			argv.actions,
			argv.groups,
			argv.projects,
			argv.maxTaskParallelization,
		).run();
	} catch (e) {
		errorHandler(e);
	}
}

export function actionsBuilder(y: Argv): Argv {
	return y
		.positional("actions", {
			required: true,
			describe: "actions to run for each project in config",
		})
		.positional("groups", {
			required: true,
			describe: "groups entry to run for specified action",
		})
		.positional("projects", {
			describe: "projects to run action on",
			default: "*",
		})
		.option("max-task-parallelization", {
			describe: "max number of parallel tasks to run",
			default: Math.ceil(os.cpus().length / 2),
		})
		.completion("completion", tabCompleteActions);
}
