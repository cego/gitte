import { loadConfig } from "../../src/config_loader";
import { Argv } from "yargs";
import { errorHandler } from "../../src/error_handler";
import { GitteCleaner } from "../../src/clean";
import { AssertionError } from "assert";
import { tabCompleteClean } from "../../src/tab_completion";

// noinspection JSUnusedGlobalSymbols
export function builder(y: Argv) {
	return cleanBuilder(y);
}
// noinspection JSUnusedGlobalSymbols
export const command = "clean [subaction]";
// noinspection JSUnusedGlobalSymbols
export const describe = "Run cleanup on projects";
// noinspection JSUnusedGlobalSymbols
export async function handler(argv: any) {
	try {
		const config = await loadConfig(argv.cwd, argv.needs, false);
		const cleaner = new GitteCleaner(config);
		switch (argv.subaction) {
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
				throw new AssertionError({
					message: `Unknown clean action: ${argv.subaction}, expected one of: untracked, local-changes, master, non-gitte`,
				});
		}
	} catch (e) {
		errorHandler(e);
	}
}

export function cleanBuilder(y: Argv): Argv {
	return y
		.positional("subaction", {
			required: false,
			describe: "The cleanup action to run. Default all",
			default: "all",
		})
		.completion("completion", (_, argv) => tabCompleteClean(argv));
}
