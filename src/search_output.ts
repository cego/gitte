import chalk from "chalk";
import { Config, SearchFor } from "./types/config";
import { ChildProcessOutput, GroupKey } from "./types/utils";
// @ts-ignore - does not have types
import template from "chalk/source/templates";
import { printHeader } from "./utils";
import tildify from "tildify";
import { TaskHandler } from "./task_running/task_handler";
import { Task, TaskState } from "./task_running/task";

export async function logTaskOutput(tasks: Task[], cwd: string): Promise<boolean> {
	let isError = false;
	for (const task of tasks) {
		if (task.state === TaskState.SKIPPED_DUPLICATE) {
			console.log(chalk`{inverse  INFO } Skipped {bold ${task.toString()}} because it was already run.`);
		} else if (task.state === TaskState.SKIPPED_FAILED_DEPENDENCY) {
			console.log(
				chalk`{bgYellow  WARN } Skipped: {bold ${task.toString()}} because it needed TODO, which failed.`,
			);
		} else if (task.state === TaskState.COMPLETED) {
			console.log(
				chalk`{bgGreen  PASS } {bold ${task.toString()}} {blue ${task.context.cmd.join(" ")}} ran in {cyan ${tildify(
					task.context.cwd ?? "",
				)}}`,
			);
		} else {
			console.error(
				chalk`{bgRed  FAIL } {bold ${task.toString()}} failed.` +
					chalk` Output logs can be found in {cyan ${tildify(await TaskHandler.getLogFilePath(cwd, task))}}`,
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
