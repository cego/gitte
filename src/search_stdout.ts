import chalk from "chalk";
import { SearchFor } from "./types/config";
import { GroupKey } from "./types/utils";
import { Output } from "promisify-child-process";

export function searchStdoutAndPrintHints(
	searchFor: SearchFor[],
	stdoutHistory: (GroupKey & Output)[],
) {
	searchFor.forEach((search) => searchForRegex(search, stdoutHistory));
}

function searchForRegex(
	searchFor: SearchFor,
	stdoutHistory: (GroupKey & Output)[],
): void {
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
