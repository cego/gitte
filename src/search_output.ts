import chalk from "chalk";
import { Config, SearchFor } from "./types/config";
// @ts-ignore - does not have types
import template from "chalk/source/templates";
import { printHeader } from "./utils";
import tildify from "tildify";
import { Task, TaskState } from "./task_running/task";
import path from "path";
import fs from "fs-extra";

export function getLogFilePath(cwd: string, task: Task): string {
	return path.join(cwd, "logs", `${task.key.project}-${task.key.action}-${task.key.group}.log`);
}

export const stashLogsToFile = (tasks: Task[], config: Config, action: string) => {
	tasks = tasks.filter((task) => task.key.action === action);
	for (const task of tasks) {
		if (!task.result) continue;
		const logsFilePath = getLogFilePath(config.cwd, task);
		const output = [task.result.out.join("")];
		output.push(
			`[exitCode] ${task.context.cmd.join(" ")} exited with ${task.result?.exitCode} in ${
				task.context.cwd
			} at ${new Date().toISOString()}`,
		);
		fs.ensureFileSync(logsFilePath);
		fs.writeFileSync(logsFilePath, output.join("\n"));
	}
};

export const sortTasksByTimeThenState = (a: Task, b: Task): number => {
	if (a.result && b.result) {
		return a.result.finishTime.getTime() - b.result.finishTime.getTime();
	}
	// sort by state: COMPLETED, FAILED before SKIPPED
	const firstStates = [TaskState.COMPLETED, TaskState.FAILED];

	if (firstStates.includes(a.state)) {
		return -1;
	}
	if (firstStates.includes(b.state)) {
		return 1;
	}

	return 0;
};

export async function logTaskOutput(tasks: Task[], cwd: string, action: string): Promise<boolean> {
	tasks = tasks.filter((task) => task.key.action === action).sort(sortTasksByTimeThenState);
	let isError = false;
	for (const task of tasks) {
		if (task.state === TaskState.SKIPPED_FAILED_DEPENDENCY) {
			console.log(
				chalk`{bgYellow  WARN } Skipped: {bold ${task.toString()}} because it needed {bold ${
					task.skippedBy ?? "unknown"
				}}, which failed.`,
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
					chalk` Output logs can be found in {cyan ${tildify(getLogFilePath(cwd, task))}}`,
			);
			isError = true;
		}
	}
	return isError;
}

export function searchOutputForHints(tasks: Task[], cfg: Config, action: string, firstHint = true) {
	tasks = tasks.filter((task) => task.key.action === action);
	tasks = tasks.sort((a, b) => {
		if (a.result && b.result) {
			return a.result.finishTime.getTime() - b.result.finishTime.getTime();
		}
		return 0;
	});

	cfg.searchFor.forEach((search) => (firstHint = searchForRegex(search, tasks, firstHint)));
	tasks.forEach((task) => {
		const searchFor = cfg.projects[task.key.project]?.actions[task.key.action]?.searchFor;
		if (searchFor) {
			searchFor.forEach((search) => (firstHint = searchForRegex(search, [task], firstHint)));
		}
	});
}

function searchForRegex(searchFor: SearchFor, tasks: Task[], firstHint: boolean): boolean {
	for (const task of tasks) {
		if (!task.result) continue;
		const outAsText = task.result.out.join("");
		if (new RegExp(searchFor.regex, "g").test(outAsText)) {
			if (firstHint) {
				printHeader("Hints");
				firstHint = false;
			}
			const groups = new RegExp(searchFor.regex, "g").exec(outAsText) ?? ([] as string[]);
			let hint: string = searchFor.hint.replace(/{(\d+)}/g, (_, d) => groups[d]);

			hint = template(chalk, hint);
			console.log(chalk`${hint} {gray (Source: ${task.toString()})}`);
		}
	}
	return firstHint;
}
