import { Config } from "./types/config";
import * as pcp from "promisify-child-process";
import path from "path";
import assert from "assert";
import dotenv from "dotenv";
import { validateYaml } from "./validate_yaml";
import fs from "fs-extra";
import yaml from "js-yaml";

export async function loadConfig(cwd: string): Promise<Config> {
	const cnfPath = `${cwd}/.git-local-devops.yml`;
	const dotenvPath = `${cwd}/.git-local-devops-env`;

	let fileContent;

	if (await fs.pathExists(dotenvPath)) {
		const envCnf = dotenv.parse(await fs.readFile(dotenvPath, "utf8"));
		assert(envCnf["REMOTE_GIT_REPO"], `REMOTE_GIT_REPO isn't defined in ${dotenvPath}`);
		assert(envCnf["REMOTE_GIT_FILE"], `REMOTE_GIT_FILE isn't defined in ${dotenvPath}`);
		assert(envCnf["REMOTE_GIT_REF"], `REMOTE_GIT_REF isn't defined in ${dotenvPath}`);
		const remoteGitProjectUrl = envCnf["REMOTE_GIT_REPO"];
		const remoteGitProjectFile = envCnf["REMOTE_GIT_FILE"];
		const remoteGitProjectRef = envCnf["REMOTE_GIT_REF"];
		const res = await pcp.spawn(
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
		// write file content to file TODO REMOVE
		await fs.writeFile(cnfPath, fileContent);
	} else if (await fs.pathExists(cnfPath)) {
		fileContent = await fs.readFile(cnfPath, "utf8");
	} else if (cwd === "/") {
		throw new Error(`No .git-local-devops.yml or .git-local-devops-env found in current or parent directories.`);
	} else {
		return loadConfig(path.resolve(cwd, ".."));
	}

	const yml: any = yaml.load(fileContent);
	assert(validateYaml(yml), "Invalid .git-local-devops.yml file");

	return yml as Config;
}
