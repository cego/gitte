import chalk from "chalk";
import { loadConfig } from "../../src/config_loader";

// noinspection JSUnusedGlobalSymbols
export const command = "validate";
// noinspection JSUnusedGlobalSymbols
export const describe = "Validate the configuration";
// noinspection JSUnusedGlobalSymbols
export async function handler(argv: any) {
	await loadConfig(argv.cwd);
	console.log(chalk`{green .git-local-devops.yml is valid}`);
}
