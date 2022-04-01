import { loadConfig } from "../../src/config_loader";
import { startup } from "../../src/startup";
import { fromConfig as gitOpsFromConfig } from "../../src/gitops";
import { fromConfig as actionsFromConfig } from "../../src/actions";
import { Argv } from "yargs";
import { errorHandler } from "../../src/error_handler";
import { actionsBuilder } from "./actions";

// noinspection JSUnusedGlobalSymbols
export function builder(y: Argv) {
	return actionsBuilder(y);
}
// noinspection JSUnusedGlobalSymbols
export const command = "run <action> <group>";
// noinspection JSUnusedGlobalSymbols
export const describe = "Run startup, git operations and actions on all projects";
// noinspection JSUnusedGlobalSymbols
export async function handler(argv: any) {
	try {
		const cnf = await loadConfig(argv.cwd);
		await startup(Object.entries(cnf.startup));
		await gitOpsFromConfig(argv.cwd, cnf);
		await actionsFromConfig(argv.cwd, cnf, argv.action, argv.group);
	} catch (e) {
		errorHandler(e);
	}
}
