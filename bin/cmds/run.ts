import { loadConfig } from "../../src/config_loader";
import { startup } from "../../src/startup";
import { fromConfig as gitOpsFromConfig } from "../../src/git-ops";
import { fromConfig as actionsFromConfig } from "../../src/actions";

// noinspection JSUnusedGlobalSymbols
export const command = "run <action> <group>";
// noinspection JSUnusedGlobalSymbols
export const describe = "Run startup, git operations and actions on all projects";
// noinspection JSUnusedGlobalSymbols
export async function handler(argv: any) {
	const cnf = await loadConfig(argv.cwd);
	await startup(Object.values(cnf.startup));
	await gitOpsFromConfig(argv.cwd, cnf);
	await actionsFromConfig(argv.cwd, cnf, argv.action, argv.group);
}
