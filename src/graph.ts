import assert from "assert";
import { Config } from "./types/config";

export type ActionGraphs = { [actionName: string]: Map<string, string[]> };

export function createActionGraphs(obj: Config): ActionGraphs {
	// find all unique action names
	const actionNames = new Set<string>();
	Object.values(obj.projects).forEach((project) => {
		Object.keys(project.actions).forEach((actionKey) => {
			actionNames.add(actionKey);
		});
	});

	// create a graph for each action
	return [...actionNames.keys()].reduce((acc, actionName) => {
		return { ...acc, [actionName]: topologicalSortActionGraph(obj, actionName) };
	}, {});
}

export function topologicalSortActionGraph(obj: Config, actionName: string, sorter = topologicalSort): string[] {
	const edges = new Map<string, string[]>();

	// Explore edges:
	Object.entries(obj.projects)
		.filter(([, project]) => project.actions[actionName])
		.forEach(([projectKey, project]) => {
			const action = project.actions[actionName];
			const needs = [...(action?.needs ?? [])];
			if (action.priority !== undefined && action.priority !== null) {
				assert(needs.length === 0, `Priority actions cannot have needs: ${projectKey}/${actionName}`);
			}
			edges.set(projectKey, needs);
		});

	return sorter(edges, actionName);
}

/**
 * https://stackoverflow.com/a/4577/17466122
 *
 * @param edges
 * @returns
 */
export function topologicalSort(edges: Map<string, string[]>, actionName: string): string[] {
	// Copy map to avoid mutations
	edges = new Map(edges);

	const sorted = [] as string[];
	const leaves = [...edges.entries()].filter(([, mapsTo]) => mapsTo.length === 0).map(([mapsFrom]) => mapsFrom);
	while (leaves.length > 0) {
		const leaf = leaves.shift() as string; // We just checked length, so this is safe
		sorted.push(leaf);
		edges.delete(leaf);
		edges.forEach((mapsTo, mapsFrom) => {
			if (mapsTo.includes(leaf)) {
				mapsTo.splice(mapsTo.indexOf(leaf), 1);
				if (mapsTo.length === 0) {
					leaves.push(mapsFrom);
				}
			}
		});
	}

	// If there are any edges left, there is a cycle
	if (edges.size > 0) {
		console.log(`Unreachable projects for action "${actionName}":`);
		// rename columns to make it easier to read
		const edgesWithNames = [...edges.entries()].map(([mapsFrom, mapsTo]) => ({ project: mapsFrom, needs: mapsTo }));
		console.table(edgesWithNames);
		assert(false, "Cycle detected in action dependencies or an action dependency is not defined");
	}

	return sorted;
}
