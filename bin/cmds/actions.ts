import { loadConfig } from "../../src/config_loader";
import { fromConfig } from "../../src/actions";
import { Argv } from "yargs";
import { errorHandler } from "../../src/error_handler";

// noinspection JSUnusedGlobalSymbols
export function builder(y: Argv) {
	return actionsBuilder(y);
}
// noinspection JSUnusedGlobalSymbols
export const command = "actions <action> <group>";
// noinspection JSUnusedGlobalSymbols
export const describe = "Run actions on all projects for <action> and <group>";
// noinspection JSUnusedGlobalSymbols
export async function handler(argv: any) {
	try {
		const cnf = await loadConfig(argv.cwd);
		await fromConfig(argv.cwd, cnf, argv.action, argv.group);
	} catch (e) {
		errorHandler(e);
	}
}

export function actionsBuilder(y: Argv): Argv {
	return y
		.positional("action", {
			required: true,
			describe: "action to run for each project in config",
		})
		.positional("group", {
			required: true,
			describe: "group entry to run for specified action",
		});
}
