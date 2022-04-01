import { loadConfig } from "../../src/config_loader";
import { fromConfig } from "../../src/gitops";
import {errorHandler} from "../../src/error_handler";

// noinspection JSUnusedGlobalSymbols
export const command = "gitops";
// noinspection JSUnusedGlobalSymbols
export const describe = "Run git operations on all projects";
// noinspection JSUnusedGlobalSymbols
export async function handler(argv: any) {
	try {
		const cnf = await loadConfig(argv.cwd);
		await fromConfig(argv.cwd, cnf);
	} catch (e) {
		errorHandler(e);
	}
}
