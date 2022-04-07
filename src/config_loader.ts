import { Config } from "./types/config";
import * as utils from "./utils";
import path from "path";
import assert, { AssertionError } from "assert";
import dotenv from "dotenv";
import { validateYaml } from "./validate_yaml";
import fs from "fs-extra";
import yaml from "js-yaml";

export async function loadConfig(cwd: string): Promise<Config> {
	const cnfPath = `${cwd}/.gitte.yml`;
	const dotenvPath = `${cwd}/.gitte-env`;

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
		return loadConfig(path.resolve(cwd, ".."));
	}

	const yml: any = yaml.load(fileContent);
	assert(validateYaml(yml), "Invalid .gitte.yml file");

	return { ...yml, cwd };
}
