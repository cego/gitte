import { loadConfig } from "../../src/config_loader";
import { Argv } from "yargs";
import { errorHandler } from "../../src/error_handler";
import { resetDisabledProjects, logDisabledProjects, toggleProjectDisable } from "../../src/disable_projects";

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
				logDisabledProjects(config);
				break;
			case "reset":
				// set disabled projects to projects which defaultDisabled
				resetDisabledProjects(config);
				break;
			default:
				toggleProjectDisable(config, argv.project);
		}
	} catch (e) {
		errorHandler(e);
	}
}

export function cleanBuilder(y: Argv): Argv {
	return y.positional("project", {
		required: false,
		describe: "The project to disable, reset to reset to default. Default: Status of disabled projects.",
		default: "status",
	});
}
