import chalk from "chalk";
import { Config, SearchFor } from "./types/config";
// @ts-ignore - does not have types
import template from "chalk/source/templates";
import { printHeader } from "./utils";
import tildify from "tildify";
import { TaskHandler } from "./task_running/task_handler";
import { Task, TaskState } from "./task_running/task";
import path from "path";
import fs from "fs-extra";

function getLogFilePath(cwd: string, task: Task): string {
	const logsFilePath = path.join(cwd, "logs", `${task.key.project}-${task.key.action}-${task.key.group}.log`);
	return logsFilePath;
};

export const stashLogsToFile = (tasks: Task[], config: Config) => {
	for (const task of tasks) {
		const logsFilePath = getLogFilePath(config.cwd, task);
		const output = [];
		output.push(...(task.result?.stdout.split("\n").map((line) => `[stdout] ${line.trim()}`) ?? []));
		output.push(...(task.result?.stderr.split("\n").map((line) => `[stderr] ${line.trim()}`) ?? []));
		output.push(
			`[exitCode] ${task.context.cmd.join(" ")} exited with ${task.result?.exitCode} in ${task.context.cwd} at ${new Date().toISOString()}`,
		);
		fs.ensureFileSync(logsFilePath);
		fs.writeFileSync(logsFilePath, output.join("\n"));
	}
};

export async function logTaskOutput(tasks: Task[], cwd: string): Promise<boolean> {
	let isError = false;
	for (const task of tasks) {
		if (task.state === TaskState.SKIPPED_DUPLICATE) {
			console.log(chalk`{inverse  INFO } Skipped {bold ${task.toString()}} because it was already run.`);
		} else if (task.state === TaskState.SKIPPED_FAILED_DEPENDENCY) {
			console.log(chalk`{bgYellow  WARN } Skipped: {bold ${task.toString()}} because it needed TODO, which failed.`);
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

export function searchOutputForHints(tasks: Task[], cfg: Config, firstHint = true) {
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
		if (
			task.result &&
			(new RegExp(searchFor.regex, "g").test(task.result.stdout) ||
				new RegExp(searchFor.regex, "g").test(task.result.stderr))
		) {
			if (firstHint) {
				printHeader("Hints");
				firstHint = false;
			}
			const groups = new RegExp(searchFor.regex, "g").exec(task.result.stdout + task.result.stderr) ?? ([] as string[]);
			let hint: string = searchFor.hint.replace(/{(\d+)}/g, (_, d) => groups[d]);

			hint = template(chalk, hint);
			console.log(chalk`${hint} {gray (Source: ${task.toString()})}`);
		}
	}
	return firstHint;
}
