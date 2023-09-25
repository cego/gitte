import { Config } from "./types/config";
import path from "path";
import assert from "assert";
import fs from "fs-extra";
import chalk from "chalk";
import { printHeader } from "./utils";

export const projectsToggleFileName = ".gitte-projects-toggled";

export function getToggledProjects(cfg: Config): { [key: string]: boolean } {
	const toggledProjectsFilePath = path.join(cfg.cwd, projectsToggleFileName);

	if (!fs.pathExistsSync(toggledProjectsFilePath)) {
		fs.writeFileSync(toggledProjectsFilePath, "", "utf8");
	}

	return fs
		.readFileSync(toggledProjectsFilePath, "utf8")
		.toString()
		.split("\n")
		.filter((x) => x.length > 0)
		.map((x) => x.split(":"))
		.reduce((carry, [projectName, enabled]) => {
			carry[projectName] = enabled === "true";
			return carry;
		}, {} as { [key: string]: boolean });
}

export function logProjectStatus(cfg: Config): void {
	const toggledProjects = getToggledProjects(cfg);

	Object.entries(cfg.projects).forEach(([projectName, project]) => {
		let enabled = !project.defaultDisabled;
		if (toggledProjects[projectName]) {
			enabled = toggledProjects[projectName];
		}

		if (enabled) {
			console.log(chalk`{bold ${projectName}:} {green enabled}`);
		} else {
			console.log(chalk`{bold ${projectName}:} {red disabled}`);
		}
	});
}

export function resetToggledProjects(cfg: Config): void {
	const toggledProjectsFilePath = path.join(cfg.cwd, projectsToggleFileName);
	fs.writeFileSync(toggledProjectsFilePath, "", "utf8");

	printHeader("Toggled projects have been cleaned.", "SUCCESS");

	logProjectStatus(cfg);
}

function saveToggledProjects(cfg: Config, toggledProjects: { [key: string]: boolean }): void {
	const toggledProjectsFilePath = path.join(cfg.cwd, projectsToggleFileName);
	fs.writeFileSync(
		toggledProjectsFilePath,
		Object.entries(toggledProjects)
			.map(([projectName, enabled]) => `${projectName}:${enabled}`)
			.join("\n"),
		"utf8",
	);
}

export function toggleProject(cfg: Config, projectName: string): void {
	// assert the project exists in config
	assert(
		cfg.projects[projectName],
		`Project "${projectName}" does not exist. (See "gitte list" to see available projects.)`,
	);

	const customToggles = getToggledProjects(cfg);
	const defaultState = !cfg.projects[projectName].defaultDisabled;
	const currentState = customToggles[projectName] ?? defaultState;
	const desiredState = !currentState;

	if (desiredState === defaultState) {
		// Delete the project from the custom toggles
		delete customToggles[projectName];
	} else {
		// Add or change the project in the custom toggles
		customToggles[projectName] = desiredState;
	}

	// Write the custom toggles to file
	saveToggledProjects(cfg, customToggles);

	if (desiredState) {
		printHeader(`${projectName} has been enabled.`, "SUCCESS");
	} else {
		printHeader(`${projectName} has been disabled.`, "SUCCESS");
	}
}
