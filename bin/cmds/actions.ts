import { loadConfig } from "../../src/config_loader";
import { Argv } from "yargs";
import { errorHandler } from "../../src/error_handler";
import { ActionOutputPrinter } from "../../src/actions_utils";

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
		const cnf = await loadConfig(argv.cwd);
		await new ActionOutputPrinter(cnf, argv.action, argv.group, argv.projects).run();
	} catch (e) {
		errorHandler(e);
	}
}

export function actionsBuilder(y: Argv): Argv {
	return y
		.positional("action", {
			required: true,
			describe: "actions to run for each project in config",
		})
		.positional("group", {
			required: true,
			describe: "groups entry to run for specified action",
		})
		.positional("projects", {
			describe: "projects to run action on",
			default: "*"
		});
}
