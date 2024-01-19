import { loadConfig } from "../../src/config_loader";
import chalk from "chalk";
import { errorHandler } from "../../src/error_handler";
import { ProjectAction } from "../../src/types/config";

// noinspection JSUnusedGlobalSymbols
export const command = "list";
// noinspection JSUnusedGlobalSymbols
export const describe = "List all projects and their actions";

function actionToPrettyString(actions: [string, ProjectAction]) {
		return chalk`{cyan <${actions[0]}>} {magenta <${Object.keys(actions[1].groups)}>}`;
}

// noinspection JSUnusedGlobalSymbols
export async function handler(argv: any) {
	try {
		const config = await loadConfig(argv.cwd as string, argv.needs, false);
		// const action_obj: Record<string, Record<string, string[]>> = {};

		for (const [name, project] of Object.entries(config.projects)) {
			console.log(chalk`{bold ${name}:} ${Object.entries(project.actions).map(actionToPrettyString)}`);
		}
	} catch (e) {
		errorHandler(e);
	}
}
