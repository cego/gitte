import { startup } from "../../src/startup";
import { loadConfig } from "../../src/config_loader";
import { errorHandler } from "../../src/error_handler";

// noinspection JSUnusedGlobalSymbols
export const command = "startup";
// noinspection JSUnusedGlobalSymbols
export const describe = "Run startup checks";
// noinspection JSUnusedGlobalSymbols
export async function handler(argv: any) {
	try {
		const cnf = await loadConfig(argv.cwd);
		await startup(Object.entries(cnf.startup));
	} catch (e) {
		errorHandler(e);
	}
}
