import { loadConfig } from "../../src/config_loader";
import { fromConfig } from "../../src/actions";

// noinspection JSUnusedGlobalSymbols
export const command = "actions <action> <group>";
// noinspection JSUnusedGlobalSymbols
export const describe = "Run actions on all projects for <action> and <group>";
// noinspection JSUnusedGlobalSymbols
export async function handler(argv: any) {
	const cnf = await loadConfig(argv.cwd);
	await fromConfig(argv.cwd, cnf, argv.action, argv.group);
}
