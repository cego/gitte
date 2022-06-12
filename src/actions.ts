import { getProjectDirFromRemote } from "./project";
import { Config, ProjectAction } from "./types/config";
import { ChildProcessOutput, GroupKey } from "./types/utils";
import { topologicalSortActionGraph } from "./graph";
import * as utils from "./utils";
import { ExecaError, ExecaReturnValue } from "execa";
import { ActionOutputPrinter } from "./actions_utils";
import _ from "lodash";

export type ActionOutput = GroupKey &
	ChildProcessOutput & { dir?: string; cmd?: string[]; wasSkippedBy?: string; wasSkippedDuplicated?: boolean };

export async function actions(
	config: Config,
	actionToRun: string,
	groupToRun: string,
	projectsToRun: string[],
	actionOutputPrinter: ActionOutputPrinter,
	runActionFn: (opts: RunActionOpts) => Promise<ActionOutput> = runAction,
): Promise<ActionOutput[]> {
	const projectsToRunActionIn = getProjectsToRunActionIn(config, actionToRun, groupToRun, projectsToRun);
	const blockedProjects = projectsToRunActionIn.filter((action) => (action.needs?.length ?? 0) > 0);
	const waitingOn = [] as string[];

	actionOutputPrinter.init(projectsToRunActionIn);

	// Go through the sorted priority groups, and run the actions
	// After an action is run, the runActionPromiseWrapper will handle calling any actions that needs the completed action.
	const stdoutBuffer: ActionOutput[] = [];
	for (const priority of uniquePriorities) {
		const runActionPromises = projectsToRunActionIn
			.filter((action) => (action.priority ?? 0) === priority && (action.needs?.length ?? 0) === 0)
			.map((action) => {
				return runActionPromiseWrapper(
					{
						config,
						keys: { project: action.project, action: action.action, group: action.group },
						actionOutputPrinter,
					},
					runActionFn,
					actionOutputPrinter,
					blockedProjects,
					waitingOn,
				);
			});

		(await Promise.all(runActionPromises)).forEach((outputArr) =>
			outputArr.forEach((output) => stdoutBuffer.push(output)),
		);
	}

	return stdoutBuffer;
}

interface RunActionOpts {
	config: Config;
	keys: GroupKey;
	actionOutputPrinter: ActionOutputPrinter;
}


export function getProjectsToRunActionIn(
	config: Config,
	actionToRun: string,
	groupToRun: string,
	projectsToRun: string[],
): (GroupKey & ProjectAction)[] {
	// get all actions from all projects with actionToRun key

	let actionsToRun = Object.entries(config.projects)
		.filter(
			([, project]) => project.actions[actionToRun]?.groups[groupToRun] || project.actions[actionToRun]?.groups["*"],
		)
		.filter(([projectName]) => {
			return projectsToRun.includes(projectName);
		})
		.reduce((carry, [projectName, project]) => {
			carry.push({
				project: projectName,
				action: actionToRun,
				group: project.actions[actionToRun]?.groups[groupToRun] ? groupToRun : "*",
				...project.actions[actionToRun],
			});

			return carry;
		}, [] as (GroupKey & ProjectAction)[]);

	/* Resolve dependencies
	 * If we want to run project A, but A needs B, we need to run B as well.
	 */
	actionsToRun = resolveDependenciesForActions(actionsToRun, config, groupToRun, actionToRun);

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
				.filter((action) => action.needs.includes(actionNoGroup))
				.forEach((action) => {
					action.needs = action.needs.filter((need) => need !== actionNoGroup);
					action.needs = [
						...(action.needs ?? []),
						...(config.projects[actionNoGroup]?.actions[actionToRun]?.needs ?? []),
					];
				});
		});

	return actionsToRun;
}

export function resolveDependenciesForActions(
	actionsToRun: (GroupKey & ProjectAction)[],
	config: Config,
	groupToRun: string,
	actionToRun: string,
) {
	actionsToRun = [
		...actionsToRun.reduce((carry, action) => {
			return new Set([...carry, ...resolveProjectDependencies(config, action)]);
		}, new Set<GroupKey & ProjectAction>()),
	]
		.filter((action) => action.groups[groupToRun] ?? action.groups["*"])
		.map((action) => {
			return {
				...action,
				group: config.projects[action.project].actions[actionToRun]?.groups[groupToRun] ? groupToRun : "*",
			};
		});

	return _.uniqBy(actionsToRun, (action) => action.project);
}

/*
 * Resolve dependencies for a key combination
 */
export function resolveProjectDependencies(
	config: Config,
	action: GroupKey & ProjectAction,
): Set<GroupKey & ProjectAction> {
	if (action.needs && action.needs.length > 0) {
		return action.needs.reduce((carry, need) => {
			const neededAction = config.projects[need].actions[action.action];
			return new Set([
				action,
				...resolveProjectDependencies(config, {
					...action,
					project: need,
					...neededAction,
				}),
				...carry,
			]);
		}, new Set<GroupKey & ProjectAction>());
	}
	return new Set<GroupKey & ProjectAction>([action]);
}
