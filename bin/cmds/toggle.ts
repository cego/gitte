import { loadConfig } from "../../src/config_loader";
import { Argv } from "yargs";
import { errorHandler } from "../../src/error_handler";
import { logProjectStatus, resetToggledProjects, toggleProject } from "../../src/toggle_projects";
import { tabCompleteToggle } from "../../src/tab_completion";

// noinspection JSUnusedGlobalSymbols
export function builder(y: Argv) {
	return cleanBuilder(y);
}
// noinspection JSUnusedGlobalSymbols
export const command = "toggle [project|reset]";
// noinspection JSUnusedGlobalSymbols
export const describe = "Toggle disabled projects";
// noinspection JSUnusedGlobalSymbols
export async function handler(argv: any) {
	try {
		const config = await loadConfig(argv.cwd, false, false);
		switch (argv.project) {
			case "status":
				// give a status of disabled projects
				logProjectStatus(config);
				break;
			case "reset":
				// set disabled projects to projects which defaultDisabled
				resetToggledProjects(config);
				break;
			default:
				toggleProject(config, argv.project);
		}
	} catch (e) {
		errorHandler(e);
	}
}

export function cleanBuilder(y: Argv): Argv {
	return y
		.positional("project", {
			required: false,
			describe: "The project to disable, reset to reset to default. Default: Status of disabled projects.",
			default: "status",
		})
		.completion("completion", (argString, yargsArgv) => {
			return tabCompleteToggle(yargsArgv);
		});
}
