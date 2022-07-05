import { loadConfig } from "../../src/config_loader";
import { y } from "../yargs";

// noinspection JSUnusedGlobalSymbols
export const command = "completion";
// noinspection JSUnusedGlobalSymbols
export const describe = "Prewarm cache and generate completion script.";
// noinspection JSUnusedGlobalSymbols
export async function handler(argv: any) {
	// load config to cache it
	await loadConfig(argv.cwd, false, false);
	y.showCompletionScript();
}
