import chalk from "chalk";
import execa from "execa";
import prompts from "prompts";
import { GroupKey } from "./types/utils";

export function printHeader(header: string) {
	console.log();
	console.log(chalk`{bgCyan  BEGIN } {bold ${header}}`);
	console.log();
}

export function spawn(file: string, args?: string[], options?: execa.Options): execa.ExecaChildProcess {
	return execa(file, args, options);
}

export function compareGroupKeys(a: GroupKey, b: GroupKey): boolean {
	return a.project === b.project && a.action === b.action && a.group === b.group;
}

export async function promptBoolean(question: string, init = false): Promise<boolean> {
	return prompts({
		type: "confirm",
		name: "value",
		message: question,
		initial: init,
	}).then((response) => response.value);
}
