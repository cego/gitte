import { default as to } from "await-to-js";
import { CmdAction, ShellAction } from "./types/config";
import * as pcp from "promisify-child-process";
import cliProgress from "cli-progress";
import chalk from "chalk";

function isCmdAction(action: CmdAction | ShellAction): action is CmdAction {
	return "cmd" in action;
}

export async function startup(startupList: [string, CmdAction | ShellAction][]) {
	const progressBar = new cliProgress.SingleBar(
		{
			format: chalk`\{bar\} \{value\}/\{total\} | Startup: {cyan \{name\}} `,
		},
		cliProgress.Presets.shades_classic,
	);
	progressBar.start(startupList.length, 0);
	let err;
	for (const [actionName, action] of startupList) {
		// Tell progress bar name of action
		progressBar.increment({ name: actionName });
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
	}
	progressBar.stop();
}
