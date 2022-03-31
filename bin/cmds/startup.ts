import { startup } from "../../src/startup";
import { loadConfig } from "../../src/config_loader";

// noinspection JSUnusedGlobalSymbols
export const command = "startup";
// noinspection JSUnusedGlobalSymbols
export const describe = "Run startup checks";
// noinspection JSUnusedGlobalSymbols
export async function handler(argv: any) {
	const cnf = await loadConfig(argv.cwd);
	await startup(Object.values(cnf.startup));
}
