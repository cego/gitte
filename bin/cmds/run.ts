import { loadConfig } from "../../src/config_loader";
import { startup } from "../../src/startup";
import { fromConfig as gitOpsFromConfig } from "../../src/gitops";
import { Argv } from "yargs";
import { errorHandler } from "../../src/error_handler";
import { actionsBuilder } from "./actions";
import { ActionOutputPrinter } from "../../src/actions_utils";

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
		await startup(cnf);
		await gitOpsFromConfig(cnf, argv.autoMerge);
		await new ActionOutputPrinter(cnf, argv.action, argv.group).run();
	} catch (e) {
		errorHandler(e);
	}
}
