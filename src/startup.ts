import {default as to} from "await-to-js";
import { asyncExec } from "./async_exec";
import { CmdAction, ShellAction } from "./validate_yaml";

export async function startup(startupList: {[key:string]: (CmdAction | ShellAction)}) {
	let err;
	for (let [_name, action] of Object.entries(startupList)) {
		if ('cmd' in action) {
			action = action as CmdAction;
			[err] = await to(asyncExec(action.cmd.join(" "), {env: process.env, encoding: "utf8"}));
			if (err) {
				err = err as any;
				err.hint = action.hint;
				throw err;
			}
		} else {
			action = action as ShellAction;
			[err] = await to(asyncExec(action.script, {shell: action.shell, env: process.env, encoding: "utf8"}));
			if (err) {
				err = err as any;
				err.hint = action.hint;
				throw err;
			}
		}
	}
}
