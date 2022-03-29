import { loadConfig } from "../../src/config_loader";
import { fromConfig } from "../../src/git-ops";

// noinspection JSUnusedGlobalSymbols
export const command = "git-ops";
// noinspection JSUnusedGlobalSymbols
export const describe = "Run git operations on all projects";
// noinspection JSUnusedGlobalSymbols
export async function handler(argv: any) {
	const cnf = await loadConfig(argv.cwd);
	await fromConfig(argv.cwd, cnf);
}
