import {default as to} from "await-to-js";
import { Utils } from "./utils";
import { CmdAction, ShellAction } from "./validate_yaml";

export async function startup(startupList: (CmdAction | ShellAction)[]) {
	let err;
	for (let action of startupList) {
		if ('cmd' in action) {
			action = action as CmdAction;
			[err] = await to(Utils.spawn(action.cmd[0], action.cmd.slice(1), {env: process.env}));
			if (err) {
				err = err as any;
				err.hint = action.hint;
				throw err;
			}
		} else {
			action = action as ShellAction;
			[err] = await to(Utils.spawn(action.script, [], {shell: action.shell, env: process.env}));
			if (err) {
				err = err as any;
				err.hint = action.hint;
				throw err;
			}
		}
	}
}