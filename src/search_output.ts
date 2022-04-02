import chalk from "chalk";
import { SearchFor } from "./types/config";
import { GroupKey } from "./types/utils";
import { Output } from "promisify-child-process";
import { ActionOutput } from "./actions";

export function logActionOutput(stdoutHistory: ActionOutput[]): void {
	for (const entry of stdoutHistory) {
		if (entry.wasSkippedBy) {
			console.log(chalk`{yellow Skipped: ${entry.project} because it needed ${entry.wasSkippedBy}, which failed. }`);
		} else if (entry.code === 0) {
			console.log(chalk`{blue ${entry.cmd?.join(" ")}} ran in {cyan ${entry.dir}}`);
		} else {
			console.error(
				chalk`"${entry.action}" "${entry.group}" {red failed}, ` +
					chalk`goto {cyan ${entry.dir}} and run {blue ${entry.cmd?.join(" ")}} manually`,
			);
		}
	}
}

export function searchOutputForHints(searchFor: SearchFor[], stdoutHistory: (GroupKey & Output)[]) {
	searchFor.forEach((search) => searchForRegex(search, stdoutHistory));
}

function searchForRegex(searchFor: SearchFor, stdoutHistory: (GroupKey & Output)[]): void {
	const regex = new RegExp(searchFor.regex);
	for (const entry of stdoutHistory) {
		if (
			(entry.stdout && regex.test(entry.stdout.toString())) ||
			(entry.stderr && regex.test(entry.stderr.toString()))
		) {
			console.log(
				chalk`{yellow Hint: ${searchFor.hint}} {gray (Source: ${entry.project}/${entry.action}/${entry.group})}`,
			);
		}
	}
}
