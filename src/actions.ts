import { getProjectDirFromRemote } from "./project";
import chalk from "chalk";
import { default as to } from "await-to-js";
import { Config, ProjectAction } from "./types/config";
import * as pcp from "promisify-child-process";
import { GroupKey, ToChildProcessOutput } from "./types/utils";
import { searchOutputForHints } from "./search_output";
import { printHeader } from "./utils";
import { getProgressBar, waitingOnToString } from "./progress";
import { SingleBar } from "cli-progress";

// TODO: Skip actions which does not have the group, but respect the need chain
// TODO: Add back the logging based of the output

export async function actions(
	config: Config,
	cwd: string,
	actionToRun: string,
	groupToRun: string,
	runActionFn: (opts: RunActionOpts) => Promise<(GroupKey & pcp.Output)> = runAction,
): Promise<(GroupKey & pcp.Output)[]> {
	const uniquePriorities = getUniquePriorities(config, actionToRun, groupToRun);
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
	const blockedActions = actions.filter((action) => action.needs?.length ?? 0 > 0);

	const progressBar = getProgressBar(`Running ${actionToRun} ${groupToRun}`);
	const waitingOn = actions.map((action) => action.project);

	progressBar.start(actions.length, 0, { status: waitingOnToString(waitingOn) });

	const stdoutBuffer: (GroupKey & pcp.Output)[] = [];
	for (const priority of uniquePriorities) {
		const runActionPromises = actions.filter(action => (action.priority ?? 0) === priority && (action.needs?.length ?? 0) === 0).map(action => {
			return runActionPromiseWrapper({ cwd, config, keys: action }, runActionFn, progressBar, blockedActions, waitingOn);

		});

		(await Promise.all(runActionPromises)).forEach((outputArr) => outputArr.forEach(output => stdoutBuffer.push(output)));
	}

	progressBar.stop();


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

export async function runActionPromiseWrapper(runActionOpts: RunActionOpts, runActionFn: (opts: RunActionOpts) => Promise<(GroupKey & pcp.Output)>, progressBar: SingleBar, blockedActions: (GroupKey & ProjectAction)[], waitingOn: string[]): Promise<(GroupKey & pcp.Output)[]> {
	return await runActionFn(runActionOpts)
		.then((res) => { 
			waitingOn.splice(waitingOn.indexOf(runActionOpts.keys.project), 1);
			progressBar.increment({ status: waitingOnToString(waitingOn) });
			return res; 
		})
		.then(async (res) => {
			blockedActions.forEach((action) => {
				action.needs = action.needs?.filter(need => need !== runActionOpts.keys.project);
			});

			const runBlockedActionPromises = blockedActions.filter(action => action.needs?.length === 0).map(action => {
				const newBlockedActions = blockedActions.filter(action => action.needs?.length !== 0);
				return runActionPromiseWrapper({ ...runActionOpts, keys: action }, runActionFn, progressBar, newBlockedActions, waitingOn);
			});

			const blockedActionsResult = (await Promise.all(runBlockedActionPromises)).reduce((carry, res) => {
				return [...carry, ...res];
			}, [] as (GroupKey & pcp.Output)[]);
			return [res, ...blockedActionsResult];
		});
}

interface RunActionOpts {
	cwd: string;
	config: Config;
	keys: GroupKey;
}

export async function runAction(options: RunActionOpts): Promise<(GroupKey & pcp.Output)> {
	const project = options.config.projects[options.keys.project];
	const group = project.actions[options.keys.action].groups[options.keys.group];
	const dir = getProjectDirFromRemote(options.cwd, project.remote);

	// console.log(chalk`{blue ${group.join(" ")}} is running in {cyan ${dir}}`);
	const [err, res]: ToChildProcessOutput = await to(
		pcp.spawn(group[0], group.slice(1), {
			cwd: dir,
			env: process.env,
			encoding: "utf8",
		}),
	);

	if (err) {
		// console.error(
		// 	chalk`"${options.keys.action}" "${options.keys.group}" {red failed}, ` +
		// 		chalk`goto {cyan ${dir}} and run {blue ${group.join(" ")}} manually`,
		// );
	}

	return {
		...options.keys,
		stdout: res?.stdout?.toString() ?? "",
		stderr: res?.stderr?.toString() ?? "",
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


