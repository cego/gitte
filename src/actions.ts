import { getProjectDirFromRemote } from "./project";
import chalk from "chalk";
import { Config, ProjectAction } from "./types/config";
import * as pcp from "promisify-child-process";
import { GroupKey } from "./types/utils";
import { logActionOutput, searchOutputForHints } from "./search_output";
import { printHeader } from "./utils";
import { getProgressBar, waitingOnToString } from "./progress";
import { SingleBar } from "cli-progress";
import { topologicalSortActionGraph } from "./graph";
import fs from "fs-extra";
import path from "path";
import execa, { ExecaError, ExecaReturnValue } from "execa";
import { Writable } from "stream";
import ansi from "ansi-escape-sequences";
import { clear } from "console";

export type ActionOutput = GroupKey & pcp.Output & { dir?: string; cmd?: string[]; wasSkippedBy?: string };

export async function actions(
	config: Config,
	actionToRun: string,
	groupToRun: string,
	runActionFn: (opts: RunActionOpts) => Promise<ActionOutput> = runAction,
): Promise<ActionOutput[]> {
	const uniquePriorities = getUniquePriorities(config, actionToRun, groupToRun);
	const actionsToRun = getActions(config, actionToRun, groupToRun);
	const blockedActions = actionsToRun.filter((action) => (action.needs?.length ?? 0) > 0);

	const progressBar = getProgressBar(`Running ${actionToRun} ${groupToRun}`);
	const waitingOn = [] as string[];

	progressBar.start(actionsToRun.length, 0, { status: waitingOnToString(waitingOn) });

	// Go through the sorted priority groups, and run the actions
	// After an action is run, the runActionPromiseWrapper will handle calling any actions that needs the completed action.
	const stdoutBuffer: ActionOutput[] = [];
	for (const priority of uniquePriorities) {
		const runActionPromises = actionsToRun
			.filter((action) => (action.priority ?? 0) === priority && (action.needs?.length ?? 0) === 0)
			.map((action) => {
				return runActionPromiseWrapper(
					{ config, keys: { project: action.project, action: action.action, group: action.group } },
					runActionFn,
					progressBar,
					blockedActions,
					waitingOn,
				);
			});

		(await Promise.all(runActionPromises)).forEach((outputArr) =>
			outputArr.forEach((output) => stdoutBuffer.push(output)),
		);
	}

	progressBar.update({ status: waitingOnToString([]) });
	progressBar.stop();
	console.log();

	logActionOutput(stdoutBuffer);
	return stdoutBuffer;
}
export function getUniquePriorities(config: Config, actionToRun: string, groupToRun: string): Set<number> {
	return Object.values(config.projects).reduce((carry, project) => {
		if (project.actions[actionToRun]?.groups[groupToRun]) {
			carry.add(project.actions[actionToRun].priority ?? 0);
		}
		return carry;
	}, new Set<number>());
}

export async function runActionPromiseWrapper(
	runActionOpts: RunActionOpts,
	runActionFn: (opts: RunActionOpts) => Promise<ActionOutput>,
	progressBar: SingleBar,
	blockedActions: (GroupKey & ProjectAction)[],
	waitingOn: string[],
): Promise<ActionOutput[]> {
	waitingOn.push(runActionOpts.keys.project);
	progressBar.update({ status: waitingOnToString(waitingOn) });
	return runActionFn(runActionOpts)
		.then((res) => {
			waitingOn.splice(waitingOn.indexOf(runActionOpts.keys.project), 1);
			progressBar.increment({ status: waitingOnToString(waitingOn) });
			return res;
		})
		.then(async (res) => {
			// if exit code was not zero, remove all blocked actions that needs this action
			const removedBlockedActions = res.code === 0 ? [] : findActionsToSkipAfterFailure(res.project, blockedActions);

			const actionsFreedtoRun = blockedActions.reduce((carry, action, i) => {
				action.needs = action.needs?.filter((need) => need !== runActionOpts.keys.project);
				if (action.needs?.length === 0) {
					delete blockedActions[i];
					carry.push(action);
				}
				return carry;
			}, [] as (GroupKey & ProjectAction)[]);

			const runBlockedActionPromises = actionsFreedtoRun.map((action) => {
				return runActionPromiseWrapper(
					{ ...runActionOpts, keys: { ...runActionOpts.keys, project: action.project } },
					runActionFn,
					progressBar,
					blockedActions,
					waitingOn,
				);
			});

			const blockedActionsResult = (await Promise.all(runBlockedActionPromises)).reduce(
				(carry, blockedActionResult) => {
					return [...carry, ...blockedActionResult];
				},
				[] as ActionOutput[],
			);
			return [res, ...blockedActionsResult, ...removedBlockedActions];
		});
}

interface RunActionOpts {
	config: Config;
	keys: GroupKey;
}

const maxLines = 10;
const lastFewLines = [] as string[];
function handleLogOutput(str: string, projectName: string, type: "stderr" | "stdout") {
	// Remove all ansi escape sequences
	str = str.replace(/\u001b[^m]*?m/g, '')
	const lines = str.split("\n");
	lastFewLines.push(...lines);

	while (lastFewLines.length > maxLines) {
		lastFewLines.shift();
	}

	process.stdout.write(ansi.cursor.nextLine(1));
	for (const line of lastFewLines) {
		process.stdout.write(ansi.cursor.nextLine(1))
		process.stdout.write(ansi.erase.inLine(2))
		process.stdout.write(chalk`{inverse  ${projectName} } {gray ${line}}`);
	}
	process.stdout.write(ansi.cursor.previousLine(lastFewLines.length+1));
}

function clearOutputLines(){
	for(let i = 0; i < maxLines; i++){
		process.stdout.write(ansi.cursor.nextLine(1))
		process.stdout.write(ansi.erase.inLine(2))
	}
	process.stdout.write(ansi.cursor.previousLine(maxLines));
}
function prepareOutputLines(){
	for(let i = 0; i < maxLines; i++){
		console.log();
	}
	process.stdout.write(ansi.cursor.hide);
	process.stdout.write(ansi.cursor.previousLine(maxLines+1));
}

export async function runAction(options: RunActionOpts): Promise<ActionOutput> {
	const project = options.config.projects[options.keys.project];
	const group = project.actions[options.keys.action].groups[options.keys.group];
	const dir = getProjectDirFromRemote(options.config.cwd, project.remote);

	const promise = execa(group[0], group.slice(1), {
		cwd: dir,
		env: process.env,
		encoding: "utf8",
		// increase max buffer from 200KB to 2MB
		maxBuffer: 1024 * 2048,
	})

	// create writable streams for stdout and stderr
	const stdout = new Writable({
		write: (chunk, encoding, callback) => {
			handleLogOutput(chunk.toString(), options.keys.project, "stdout");
			callback();
		},
	});
	const stderr = new Writable({
		write: (chunk, encoding, callback) => {
			handleLogOutput(chunk.toString(), options.keys.project, "stderr");
			callback();
		},
	});

	promise.stdout?.pipe(stdout);
	promise.stderr?.pipe(stderr);

	const res: ExecaReturnValue<string> | ExecaError<string> = await promise.catch((err) => err);

	return {
		...options.keys,
		stdout: res.stdout?.toString() ?? "",
		stderr: res.stderr?.toString() ?? "",
		code: res.exitCode,
		signal: res.signal,
		dir,
		cmd: group,
	};
}

export async function fromConfig(cnf: Config, actionToRun: string, groupToRun: string) {
	printHeader("Running actions");

	prepareOutputLines();
	const stdoutBuffer: (GroupKey & pcp.Output)[] = await actions(cnf, actionToRun, groupToRun);
	clearOutputLines();
	if (cnf.searchFor) searchOutputForHints(cnf, stdoutBuffer);
	if (stdoutBuffer.length === 0) {
		console.log(chalk`{yellow No groups found for action {cyan ${actionToRun}} and group {cyan ${groupToRun}}}`);
	}
	fs.writeFileSync(path.join(cnf.cwd, ".output.json"), JSON.stringify(stdoutBuffer));
}

export function getActions(config: Config, actionToRun: string, groupToRun: string): (GroupKey & ProjectAction)[] {
	// get all actions from all projects with actionToRun key
	const actionsToRun = Object.entries(config.projects)
		.filter(([, project]) => project.actions[actionToRun]?.groups[groupToRun])
		.reduce((carry, [projectName, project]) => {
			carry.push({
				project: projectName,
				action: actionToRun,
				group: groupToRun,
				...project.actions[actionToRun],
			});

			return carry;
		}, [] as (GroupKey & ProjectAction)[]);

	/**
	 * Sometime an action will not have the specific group it needs to run.
	 * If another action needs such action, we have to rearrange dependencies, as such action cannot be run without the specific group.
	 * This is done by adding the needs from the action without the specific group, to the actions that need the action witohut the specific group
	 *
	 * For example:
	 *
	 * A needs B, B needs C
	 *
	 * We want to run group X, but only A and C have group X.
	 * In this case we rewrite A to needs C.
	 *
	 * In order to avoid having multiple iterations, we sort the actions topologically first to ensure the dependencies are always resolved.
	 */
	topologicalSortActionGraph(config, actionToRun)
		.reverse()
		.filter((action) => !config.projects[action].actions[actionToRun]?.groups[groupToRun])
		.forEach((actionNoGroup) => {
			// Find all actions that needs this action, remove this action from the needs list, and replace it with the needs of this action
			actionsToRun
				.filter((action) => action.needs?.includes(actionNoGroup))
				.forEach((action) => {
					action.needs = action.needs?.filter((need) => need !== actionNoGroup);
					action.needs = [
						...(action.needs ?? []),
						...(config.projects[actionNoGroup]?.actions[actionToRun]?.needs ?? []),
					];
				});
		});

	return actionsToRun;
}
export function findActionsToSkipAfterFailure(
	failedProject: string,
	blockedActions: (GroupKey & ProjectAction)[],
): ActionOutput[] {
	const blockedActionsSkipped = [] as ActionOutput[];
	const actionsToSkip = blockedActions.filter((actionToSkip) => actionToSkip.needs?.includes(failedProject));
	actionsToSkip.forEach((actionToSkip) => {
		blockedActionsSkipped.push({
			...actionToSkip,
			wasSkippedBy: failedProject,
		});
		delete blockedActions[blockedActions.indexOf(actionToSkip)];
		findActionsToSkipAfterFailure(actionToSkip.project, blockedActions).forEach((skippedActionResult) => {
			blockedActionsSkipped.push(skippedActionResult);
		});
	});
	return blockedActionsSkipped;
}
