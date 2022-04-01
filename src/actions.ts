import { getProjectDirFromRemote } from "./project";
import chalk from "chalk";
import { Config, ProjectAction } from "./types/config";
import * as pcp from "promisify-child-process";
import { ActionOutput, GroupKey } from "./types/utils";
import { logActionOutput, searchOutputForHints } from "./search_output";
import { printHeader } from "./utils";
import { getProgressBar, waitingOnToString } from "./progress";
import { SingleBar } from "cli-progress";
import { topologicalSortActionGraph } from "./graph";

export async function actions(
	config: Config,
	cwd: string,
	actionToRun: string,
	groupToRun: string,
	runActionFn: (opts: RunActionOpts) => Promise<ActionOutput> = runAction,
): Promise<ActionOutput[]> {
	const uniquePriorities = getUniquePriorities(config, actionToRun, groupToRun);
	const actions = getActions(config, actionToRun, groupToRun);
	const blockedActions = actions.filter((action) => action.needs?.length ?? 0 > 0);

	const progressBar = getProgressBar(`Running ${actionToRun} ${groupToRun}`);
	const waitingOn = [] as string[];

	progressBar.start(actions.length, 0, { status: waitingOnToString(waitingOn) });

	const stdoutBuffer: ActionOutput[] = [];
	for (const priority of uniquePriorities) {
		const runActionPromises = actions
			.filter((action) => (action.priority ?? 0) === priority && (action.needs?.length ?? 0) === 0)
			.map((action) => {
				return runActionPromiseWrapper(
					{ cwd, config, keys: action },
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
function getUniquePriorities(config: Config, actionToRun: string, groupToRun: string): Set<number> {
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
	return await runActionFn(runActionOpts)
		.then((res) => {
			waitingOn.splice(waitingOn.indexOf(runActionOpts.keys.project), 1);
			progressBar.increment();
			return res;
		})
		.then(async (res) => {
			blockedActions.forEach((action) => {
				action.needs = action.needs?.filter((need) => need !== runActionOpts.keys.project);
			});

			const runBlockedActionPromises = blockedActions
				.filter((action) => action.needs?.length === 0)
				.map((action) => {
					const newBlockedActions = blockedActions.filter((action) => action.needs?.length !== 0);
					return runActionPromiseWrapper(
						{ ...runActionOpts, keys: action },
						runActionFn,
						progressBar,
						newBlockedActions,
						waitingOn,
					);
				});

			const blockedActionsResult = (await Promise.all(runBlockedActionPromises)).reduce((carry, res) => {
				return [...carry, ...res];
			}, [] as ActionOutput[]);
			return [res, ...blockedActionsResult];
		});
}

interface RunActionOpts {
	cwd: string;
	config: Config;
	keys: GroupKey;
}

export async function runAction(options: RunActionOpts): Promise<ActionOutput> {
	const project = options.config.projects[options.keys.project];
	const group = project.actions[options.keys.action].groups[options.keys.group];
	const dir = getProjectDirFromRemote(options.cwd, project.remote);

	const res = await pcp
		.spawn(group[0], group.slice(1), {
			cwd: dir,
			env: process.env,
			encoding: "utf8",
		})
		.catch((err) => err);

	return {
		...options.keys,
		stdout: res.stdout?.toString() ?? "",
		stderr: res.stderr?.toString() ?? "",
		code: res.code,
		dir,
		cmd: group,
	};
}

export async function fromConfig(cwd: string, cnf: Config, actionToRun: string, groupToRun: string) {
	printHeader("Running actions");
	const stdoutBuffer: (GroupKey & pcp.Output)[] = await actions(cnf, cwd, actionToRun, groupToRun);
	if (cnf.searchFor) searchOutputForHints(cnf.searchFor, stdoutBuffer);
	if (stdoutBuffer.length === 0) {
		console.log(chalk`{yellow No groups found for action {cyan ${actionToRun}} and group {cyan ${groupToRun}}}`);
	}
}

function getActions(config: Config, actionToRun: string, groupToRun: string): (GroupKey & ProjectAction)[] {
	// get all actions from all projects with actionToRun key
	const actions = Object.entries(config.projects)
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

	// Find all actions that doesn't have the group, and fix the needs chain for actions that needs that action
	// Use topological sort in order to only iterate once. createActionGra
	topologicalSortActionGraph(config, actionToRun)
		.filter((action) => !config.projects[action].actions[actionToRun]?.groups[groupToRun])
		.forEach((actionNoGroup) => {
			// Find all actions that needs this action, remove this action from the needs list, and replace it with the needs of this action
			actions
				.filter((action) => action.needs?.includes(actionNoGroup))
				.forEach((action) => {
					action.needs = action.needs?.filter((need) => need !== actionNoGroup);
					action.needs = [
						...(action.needs ?? []),
						...(config.projects[actionNoGroup]?.actions[actionToRun]?.needs ?? []),
					];
				});
		});

	return actions;
}
