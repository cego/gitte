import {default as to} from "await-to-js";
import execa from 'execa';
import { CmdAction, ShellAction } from "./validate_yaml";

export async function startup(startupList: {[key:string]: (CmdAction | ShellAction)}) {
	let err;
	for (let [_name, action] of Object.entries(startupList)) {
		if ('cmd' in action) {
			action = action as CmdAction;
			[err] = await to(execa(action.cmd[0], action.cmd.splice(1), {env: process.env, encoding: "utf8"}));
			if (err) {
				err = err as any;
				err.hint = action.hint;
				throw err;
			}
		} else {
			action = action as ShellAction;
			[err] = await to(execa(action.script, [], {shell: action.shell, env: process.env, encoding: "utf8"}));
			if (err) {
				err = err as any;
				err.hint = action.hint;
				throw err;
			}
		}
	}
}