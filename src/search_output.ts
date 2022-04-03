import chalk from "chalk";
import { Config, SearchFor } from "./types/config";
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

export function searchOutputForHints(cfg: Config, stdoutHistory: (GroupKey & Output)[]) {
	cfg.searchFor.forEach((search) => searchForRegex(search, stdoutHistory));
	stdoutHistory.forEach((entry) => {
		const searchFor = cfg.projects[entry.project]?.actions[entry.action]?.searchFor;
		if (searchFor) {
			searchFor.forEach((search) => searchForRegex(search, [entry]));
		}
	});
}

function searchForRegex(searchFor: SearchFor, stdoutHistory: (GroupKey & Output)[]): void {
	for (const entry of stdoutHistory) {
		if (
			(entry.stdout && new RegExp(searchFor.regex, "g").test(entry.stdout.toString())) ||
			(entry.stderr && new RegExp(searchFor.regex, "g").test(entry.stderr.toString()))
		) {
			const groups =
				new RegExp(searchFor.regex, "g").exec(entry.stdout?.toString() ?? entry.stderr?.toString() ?? "") ??
				([] as string[]);
			let hint: string = searchFor.hint.replace(/{(\d+)}/g, (_, d) => groups[d]);

			/*
			 * Yes this is a hack, but it works for now untill we upgrade to ESM and chalkTemplate
			 * Yes this makes it possible for the config to execute arbitary javascript.
			 * - The config already has access to define all the commands we execute, so this is nothing new.
			 */
			hint = eval(`require("chalk")\`${hint}\``);
			console.log(chalk`{inverse  INFO } ${hint} {gray (Source: ${entry.project})}`);
		}
	}
}
