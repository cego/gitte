import { default as to } from "await-to-js";
import { CmdAction, Config, ShellAction } from "./types/config";
import { applyPromiseToEntriesWithProgressBarSync } from "./progress";
import * as utils from "./utils";
import { ErrorWithHint } from "./types/utils";

function isCmdAction(action: CmdAction | ShellAction): action is CmdAction {
	return "cmd" in action;
}

export async function startup(cnf: Config) {
	utils.printHeader("Startup checks");

	const startupList = Object.entries(cnf.startup);

	const fn = async (action: CmdAction | ShellAction) => {
		let err;
		if (isCmdAction(action)) {
			[err] = await to(
				utils.spawn(action.cmd[0], action.cmd.slice(1), {
					env: process.env,
					encoding: "utf8",
					cwd: cnf.cwd,
				}),
			);
			if (err) {
				if (action.hint) throw new ErrorWithHint(action.hint, err);
				throw err;
			}
		} else {
			[err] = await to(
				utils.spawn(action.script, [], {
					shell: action.shell,
					env: process.env,
					encoding: "utf8",
					cwd: cnf.cwd,
				}),
			);
			if (err) {
				if (action.hint) throw new ErrorWithHint(action.hint, err);
				throw err;
			}
		}
	};

	await applyPromiseToEntriesWithProgressBarSync("Startup checks", startupList, fn);
}
