import { loadConfig } from "../../src/config_loader";
import { Argv } from "yargs";
import { errorHandler } from "../../src/error_handler";
import { TaskHandler } from "../../src/task_running/task_handler";
import { GitteCleaner } from "../../src/clean";

// noinspection JSUnusedGlobalSymbols
export function builder(y: Argv) {
	return cleanBuilder(y);
}
// noinspection JSUnusedGlobalSymbols
export const command = "clean [untracked|local-changes|master|non-gitte]";
// noinspection JSUnusedGlobalSymbols
export const describe = "Run cleanup on projects";
// noinspection JSUnusedGlobalSymbols
export async function handler(argv: any) {
	try {
        const config = await loadConfig(argv.cwd);
        const cleaner = new GitteCleaner(config)
		switch(argv.cleanAction) {
            case "untracked":
                await cleaner.cleanUntracked();
                break;
            case "local-changes":
                await cleaner.cleanLocalChanges();
                break;
            case "master":
                await cleaner.cleanMaster();
                break;
            case "non-gitte":
                await cleaner.cleanNonGitte();
                break;
            case "all":
                await cleaner.clean();
                break;
            default:
                throw new Error("Unknown clean action");
        }
	} catch (e) {
		errorHandler(e);
	}
}

export function cleanBuilder(y: Argv): Argv {
	return y
		.positional("cleanAction", {
			required: false,
			describe: "The cleanup action to run. Default all",
            default: "all",
		})
}
