import { Config } from "./types/config";
import * as utils from "./utils";
import path from "path";
import assert, { AssertionError } from "assert";
import dotenv from "dotenv";
import { validateYaml } from "./validate_yaml";
import fs from "fs-extra";
import yaml from "js-yaml";
import * as _ from "lodash";
import { getDisabledProjects, getPreviouslySeenProjectsFromCache } from "./disable_projects";
import { Cache } from "./cache";

export async function loadConfig(cwd: string, needs = true, shouldDisableProjects = true): Promise<Config> {
	const cnfPath = path.join(cwd, `.gitte.yml`);
	const dotenvPath = path.join(cwd, `.gitte-env`);
	const overridePath = path.join(cwd, ".gitte-override.yml");
	const cachePath = path.join(cwd, ".gitte-cache.json");
	const projectsDisablePath = path.join(cwd, ".gitte-projects-disable");

	let fileContent;

	if (await fs.pathExists(dotenvPath)) {
		const envCnf = dotenv.parse(await fs.readFile(dotenvPath, "utf8"));
		assert(envCnf["REMOTE_GIT_REPO"], `REMOTE_GIT_REPO isn't defined in ${dotenvPath}`);
		assert(envCnf["REMOTE_GIT_FILE"], `REMOTE_GIT_FILE isn't defined in ${dotenvPath}`);
		assert(envCnf["REMOTE_GIT_REF"], `REMOTE_GIT_REF isn't defined in ${dotenvPath}`);
		const remoteGitProjectUrl = envCnf["REMOTE_GIT_REPO"];
		const remoteGitProjectFile = envCnf["REMOTE_GIT_FILE"];
		const remoteGitProjectRef = envCnf["REMOTE_GIT_REF"];
		const res = await utils.spawn(
			"git",
			[
				"archive",
				`--remote=${remoteGitProjectUrl}`,
				remoteGitProjectRef,
				remoteGitProjectFile,
				"|",
				"tar",
				"-xO",
				remoteGitProjectFile,
			],
			{ shell: "bash", cwd, env: process.env, encoding: "utf8" },
		);
		fileContent = `${res.stdout}`;
	} else if (await fs.pathExists(cnfPath)) {
		fileContent = await fs.readFile(cnfPath, "utf8");
	} else if (cwd === "/") {
		const message = `No .gitte.yml or .gitte-env found in current or parent directories.`;
		throw new AssertionError({ message });
	} else {
		return loadConfig(path.resolve(cwd, ".."), needs, shouldDisableProjects);
	}

	// Load .gitte-override.yml
	let yml: any = yaml.load(fileContent);
	if (await fs.pathExists(overridePath)) {
		const overrideContent = await fs.readFile(overridePath, "utf8");
		const overrideYml: any = yaml.load(overrideContent);
		yml = _.merge(yml, overrideYml);
	}

	const seenProjects = getPreviouslySeenProjectsFromCache(cachePath);

	// Write .gitte-cache.json
	const cache: Cache = {
		version: 1,
		seenProjects: Object.keys(yml.projects),
		config: yml,
	};
	fs.writeJsonSync(cachePath, cache, { spaces: 4 });

	const disabledProjects = getDisabledProjects(seenProjects, projectsDisablePath, yml);
	if (shouldDisableProjects) {
		disabledProjects.forEach((projectName) => {
			_.unset(yml.projects, projectName);
		});
	}

	assert(validateYaml(yml), "Invalid .gitte.yml file");

	// For any action, replace needs with an empty array if undefined.
	Object.entries(yml.projects).forEach(([, project]) => {
		Object.entries(project.actions).forEach(([, action]) => {
			action.needs = action.needs || [];
			action.priority = action.priority || null;
			action.searchFor = action.searchFor || [];
		});
	});

	return { ...yml, cwd, needs };
}
