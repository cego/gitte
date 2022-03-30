import { loadConfig } from "../../src/config_loader";
import { fromConfig } from "../../src/actions";
import { Argv } from "yargs";

// noinspection JSUnusedGlobalSymbols
export function builder(y: Argv) {
	return y
		.positional("action", {
			required: false,
			describe: "action to run for each project in config",
		})
		.positional("group", {
			required: false,
			describe: "group entry to run for specified action",
		});
}
// noinspection JSUnusedGlobalSymbols
export const command = "actions <action> <group>";
// noinspection JSUnusedGlobalSymbols
export const describe = "Run actions on all projects for <action> and <group>";
// noinspection JSUnusedGlobalSymbols
export async function handler(argv: any) {
	const cnf = await loadConfig(argv.cwd);
	await fromConfig(argv.cwd, cnf, argv.action, argv.group);
}
