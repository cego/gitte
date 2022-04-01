import { loadConfig } from "../../src/config_loader";
import chalk from "chalk";
import {errorHandler} from "../../src/error_handler";

// noinspection JSUnusedGlobalSymbols
export const command = "list";
// noinspection JSUnusedGlobalSymbols
export const describe = "List all projects and their actions";
// noinspection JSUnusedGlobalSymbols
export async function handler(argv: any) {
	try {
		const config = await loadConfig(argv.cwd as string);
		for (const [name, project] of Object.entries(config.projects)) {
			console.log(chalk`{bold ${name}:} {cyan [${Object.keys(project.actions).join(", ")}]}`);
		}
	} catch (e) {
		errorHandler(e);
	}
}
