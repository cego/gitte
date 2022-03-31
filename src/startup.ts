import { default as to } from "await-to-js";
import { CmdAction, ShellAction } from "./types/config";
import * as pcp from "promisify-child-process";
import { printHeader } from "./utils";
import { applyPromiseToEntriesWithProgressBarSync } from "./progress";

function isCmdAction(action: CmdAction | ShellAction): action is CmdAction {
	return "cmd" in action;
}

export async function startup(startupList: [string, CmdAction | ShellAction][]) {
	printHeader("Startup checks");

	const fn = async (action: CmdAction | ShellAction) => {
		let err;
		if (isCmdAction(action)) {
			[err] = await to(
				pcp.spawn(action.cmd[0], action.cmd.slice(1), {
					env: process.env,
					encoding: "utf8",
				}),
			);
			if (err) {
				err = err as any;
				err.hint = action.hint;
				throw err;
			}
		} else {
			[err] = await to(
				pcp.spawn(action.script, [], {
					shell: action.shell,
					env: process.env,
					encoding: "utf8",
				}),
			);
			if (err) {
				err = err as any;
				err.hint = action.hint;
				throw err;
			}
		}
	};

	await applyPromiseToEntriesWithProgressBarSync("Startup checks", startupList, fn);
}
