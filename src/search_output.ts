import chalk from "chalk";
import { Config, SearchFor } from "./types/config";
import { ChildProcessOutput, GroupKey } from "./types/utils";
import { ActionOutput } from "./actions";
// @ts-ignore - does not have types
import template from "chalk/source/templates";
import { printHeader } from "./utils";
import tildify from "tildify";
import { ActionOutputPrinter } from "./actions_utils";

export async function logActionOutput(stdoutHistory: ActionOutput[], cwd: string): Promise<boolean> {
	let isError = false;
	for (const entry of stdoutHistory) {
		if (entry.wasSkippedDuplicated) {
			console.log(chalk`{inverse  INFO } Skipped {bold ${entry.project}} because it was already run.`);
		} else if (entry.wasSkippedBy) {
			console.log(
				chalk`{bgYellow  WARN } Skipped: {bold ${entry.project}} because it needed ${entry.wasSkippedBy}, which failed.`,
			);
		} else if (entry.exitCode === 0) {
			console.log(
				chalk`{bgGreen  PASS } {bold ${entry.project}} {blue ${entry.cmd?.join(" ")}} ran in {cyan ${tildify(
					entry.dir ?? "",
				)}}`,
			);
		} else {
			console.error(
				chalk`{bgRed  FAIL } {bold ${entry.project}} failed running ${entry.action} ${entry.group}.` +
					chalk `Output logs can be found in ${await ActionOutputPrinter.getLogFilePath(cwd, entry)}`
			);
			isError = true;
		}
	}
	return isError;
}

export function searchOutputForHints(cfg: Config, stdoutHistory: (GroupKey & ChildProcessOutput)[], firstHint = true) {
	cfg.searchFor.forEach((search) => (firstHint = searchForRegex(search, stdoutHistory, firstHint)));
	stdoutHistory.forEach((entry) => {
		const searchFor = cfg.projects[entry.project]?.actions[entry.action]?.searchFor;
		if (searchFor) {
			searchFor.forEach((search) => (firstHint = searchForRegex(search, [entry], firstHint)));
		}
	});
}

function searchForRegex(
	searchFor: SearchFor,
	stdoutHistory: (GroupKey & ChildProcessOutput)[],
	firstHint: boolean,
): boolean {
	for (const entry of stdoutHistory) {
		if (
			(entry.stdout && new RegExp(searchFor.regex, "g").test(entry.stdout.toString())) ||
			(entry.stderr && new RegExp(searchFor.regex, "g").test(entry.stderr.toString()))
		) {
			if (firstHint) {
				printHeader("Hints");
				firstHint = false;
			}
			const groups =
				new RegExp(searchFor.regex, "g").exec(entry.stdout?.toString() ?? entry.stderr?.toString() ?? "") ??
				([] as string[]);
			let hint: string = searchFor.hint.replace(/{(\d+)}/g, (_, d) => groups[d]);

			hint = template(chalk, hint);
			console.log(chalk`${hint} {gray (Source: ${entry.project})}`);
		}
	}
	return firstHint;
}
