import { Config } from "./types/config";
import path from "path";
import assert from "assert";
import fs from "fs-extra";
import { loadCachePath } from "./cache";
import chalk from "chalk";
import { printHeader } from "./utils";

export function getDisabledProjects(seenProjects: string[], projectsDisablePath: string, cfg: Config): string[] {
	// Load .gitte-projects-disable
	if (!fs.pathExistsSync(projectsDisablePath)) {
		fs.writeFileSync(projectsDisablePath, "", "utf8");
	}

	const projectsDisabled: string[] = fs
		.readFileSync(projectsDisablePath, "utf8")
		.toString()
		.split("\n")
		.filter((x) => x.length > 0);

	// Disable projects that were not previously seen and have "defaultDisabled" set to true
	Object.entries(cfg.projects).forEach(([projectName, project]) => {
		if (project.defaultDisabled && !seenProjects.includes(projectName) && !projectsDisabled.includes(projectName)) {
			projectsDisabled.push(projectName);
		}
	});

	// Write .gitte-projects-disable
	fs.writeFileSync(projectsDisablePath, projectsDisabled.join("\n"), "utf8");

	return projectsDisabled;
}

export function getPreviouslySeenProjectsFromCache(cachePath: string): string[] {
	const cache = loadCachePath(cachePath);
	if (cache !== null) {
		return cache.seenProjects;
	}
	return [];
}

export function logDisabledProjects(cfg: Config): void {
	const cwd = cfg.cwd;
	const cachePath = path.join(cwd, ".gitte-cache.json");
	const projectsDisablePath = path.join(cwd, ".gitte-projects-disable");

	const seenProjects = getPreviouslySeenProjectsFromCache(cachePath);
	const projectsDisabled = getDisabledProjects(seenProjects, projectsDisablePath, cfg);

	Object.keys(cfg.projects).forEach((projectName) => {
		if (projectsDisabled.includes(projectName)) {
			console.log(chalk`{bold ${projectName}:} {red disabled}`);
		} else {
			console.log(chalk`{bold ${projectName}:} {green enabled}`);
		}
	});
}

export function resetDisabledProjects(cfg: Config): void {
	// overwrite .gitte-projects-disable with defaultDisabled projects
	const cwd = cfg.cwd;
	const projectsDisablePath = path.join(cwd, ".gitte-projects-disable");

	const defaultDisabledProjects = Object.entries(cfg.projects).reduce((carry, [projectName, project]) => {
		if (project.defaultDisabled) {
			return [...carry, projectName];
		}
		return carry;
	}, [] as string[]);

	fs.writeFileSync(projectsDisablePath, defaultDisabledProjects.join("\n"), "utf8");

	printHeader("Disabled projects have been cleaned.", "SUCCESS");

	logDisabledProjects(cfg);
}

export function toggleProjectDisable(cfg: Config, projectName: string): void {
	const cwd = cfg.cwd;
	const cachePath = path.join(cwd, ".gitte-cache.json");
	const projectsDisablePath = path.join(cwd, ".gitte-projects-disable");

	// assert the project exists in config
	assert(
		cfg.projects[projectName],
		`Project "${projectName}" does not exist. (See "gitte list" to see available projects.)`,
	);

	const seenProjects = getPreviouslySeenProjectsFromCache(cachePath);
	const projectsDisabled = getDisabledProjects(seenProjects, projectsDisablePath, cfg);

	let enabled = true;

	if (projectsDisabled.includes(projectName)) {
		projectsDisabled.splice(projectsDisabled.indexOf(projectName), 1);
	} else {
		projectsDisabled.push(projectName);
		enabled = false;
	}

	fs.writeFileSync(projectsDisablePath, projectsDisabled.join("\n"), "utf8");

	if (enabled) {
		printHeader(`${projectName} has been enabled.`, "SUCCESS");
	} else {
		printHeader(`${projectName} has been disabled.`, "SUCCESS");
	}
}
