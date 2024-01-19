import { loadConfig } from "../../src/config_loader";
import chalk from "chalk";
import { errorHandler } from "../../src/error_handler";

// noinspection JSUnusedGlobalSymbols
export const command = "list";
// noinspection JSUnusedGlobalSymbols
export const describe = "List all projects and their actions";
// noinspection JSUnusedGlobalSymbols
export async function handler(argv: any) {
	try {
		const config = await loadConfig(argv.cwd as string, argv.needs, false);
		let action_obj: Record<string, Record<string, string[]>> = {};

		for (const [name, project] of Object.entries(config.projects)) {
			const groups = Object.keys(project.actions);

			// instantiate
			action_obj[name] = {};

			groups.forEach(function (group) {
				action_obj[name][group] = Object.keys(project.actions[group].groups);
			});
		}

		for (const name in action_obj) {
			let group_action = "";
			let i = Object.keys(action_obj[name]).length;

			for (const group in action_obj[name]) {
				if (i <= 1) {
					group_action += chalk`{cyan <${group}>} {magenta <${action_obj[name][group]}>}`;
				} else {
					group_action += chalk`{cyan <${group}>} {magenta <${action_obj[name][group]}>}, `;
				}
				--i;
			}
			console.log(chalk`{bold ${name}:} ${group_action}`);
		}
	} catch (e) {
		errorHandler(e);
	}
}
