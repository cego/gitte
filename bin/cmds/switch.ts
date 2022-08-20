import { loadConfig } from "../../src/config_loader";
import { Argv } from "yargs";
import { errorHandler } from "../../src/error_handler";
import { TaskHandler } from "../../src/task_running/task_handler";
import os from "os";
import { TaskPlanner } from "../../src/task_running/task_planner";
import assert from "assert";
import { fromConfig as gitOpsFromConfig } from "../../src/gitops";
import { startup } from "../../src/startup";
import { tabCompleteSwitch } from "../../src/tab_completion";

// noinspection JSUnusedGlobalSymbols
export function builder(y: Argv) {
	return y
		.positional("groups", {
			required: true,
			describe: "groups entry to run for specified action",
		})
		.option("max-task-parallelization", {
			describe: "max number of parallel tasks to run",
			default: Math.ceil(os.cpus().length / 2),
		})
		.completion("completion", tabCompleteSwitch);
	// todo completion
}
// noinspection JSUnusedGlobalSymbols
export const command = "switch <groups>";
// noinspection JSUnusedGlobalSymbols
export const describe = "Downs every group except for the specified one, ups the specified one.";
// noinspection JSUnusedGlobalSymbols
export async function handler(argv: any) {
	try {
		const cnf = await loadConfig(argv.cwd, argv.needs);
		assert(cnf.switch !== undefined, "config must have switch section");
		await startup(cnf);
		await gitOpsFromConfig(cnf, argv.autoMerge);

		const groupsToUp = argv.groups.split("+");
		// down all groups except the ones to up
		const allGroups = Object.values(cnf.projects).reduce((acc, project) => {
			Object.values(project.actions).forEach((action) => {
				Object.keys(action.groups).forEach((group) => {
					acc.add(group);
				});
			});
			return acc;
		}, new Set<string>());
		const groupsToDown = Array.from(allGroups).filter((group) => !groupsToUp.includes(group) && group !== "*");
		const taskPlanner = new TaskPlanner(cnf);
		const notCommonProjects = Object.entries(cnf.projects)
			.filter((entry) => !entry[1].common)
			.map((entry) => entry[0]);
		const plan = [
			...taskPlanner.plan([cnf.switch.downAction], groupsToDown, notCommonProjects),
			...taskPlanner.plan([cnf.switch.upAction], groupsToUp, notCommonProjects),
		];
		await new TaskHandler(cnf, plan, [cnf.switch.downAction, cnf.switch.upAction], argv.maxTaskParallelization).run();
	} catch (e) {
		errorHandler(e);
	}
}
