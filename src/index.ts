import fs from "fs-extra";
import yaml from "js-yaml";
import { runActions } from "./actions";
import { gitOperations } from "./git_operations";
import assert from "assert";
import { startup } from "./startup";
import dotenv from "dotenv";
import { validateYaml } from "./validate_yaml";
import { getPriorityRange } from "./priority";
import { Config } from "./types/config";
import * as pcp from "promisify-child-process";


export async function start(cwd: string, actionToRun: string, groupToRun: string) {
	const cnfPath = `${cwd}/.git-local-devops.yml`;
	const dotenvPath = `${cwd}/.git-local-devops-env`;

	let fileContent;

	if (await fs.pathExists(dotenvPath)) {
		const envCnf = dotenv.parse(await fs.readFile(dotenvPath)); // will return an object
		assert(envCnf['REMOTE_GIT_REPO'], `REMOTE_GIT_REPO isn't defined in ${dotenvPath}`);
		assert(envCnf['REMOTE_GIT_FILE'], `REMOTE_GIT_FILE isn't defined in ${dotenvPath}`);
		assert(envCnf['REMOTE_GIT_REF'], `REMOTE_GIT_REF isn't defined in ${dotenvPath}`);
		const remoteGitProjectUrl = envCnf['REMOTE_GIT_REPO'];
		const remoteGitProjectFile = envCnf['REMOTE_GIT_FILE'];
		const remoteGitProjectRef = envCnf['REMOTE_GIT_REF'];
		const res = await pcp.spawn(
			"git", ["archive", `--remote=${remoteGitProjectUrl}`, remoteGitProjectRef, remoteGitProjectFile, "|", "tar", "-xO", remoteGitProjectFile],
			{shell: "bash", cwd, env: process.env, encoding: "utf8"},
		);
		fileContent = `${res.stdout}`;
	} else {
		assert(await fs.pathExists(cnfPath), `${cwd} doesn't contain an .git-local-devops.yml file`);
		fileContent = await fs.readFile(`${cwd}/.git-local-devops.yml`, "utf8");
	}

	const yml: any = yaml.load(fileContent);
	assert(validateYaml(yml), "Invalid .git-local-devops.yml file");
	const cnf: Config = yml;

	await startup(Object.values(cnf.startup));

	const gitOperationsPromises = [];
	for (const projectObj of Object.values(cnf.projects)) {
		gitOperationsPromises.push(gitOperations(cwd, projectObj));

	}
	await Promise.all(gitOperationsPromises);

	const prioRange = getPriorityRange(Object.values(cnf.projects));

	for (let i = prioRange.min; i < prioRange.max; i++) {
		const runActionPromises = [];
		for (const projectObj of Object.values(cnf.projects)) {
			runActionPromises.push(runActions(cwd, projectObj, i, actionToRun, groupToRun));
		}
		await Promise.all(runActionPromises);
	}
}
