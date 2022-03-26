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
import chalk from "chalk";
import * as pcp from "promisify-child-process";


export async function start(cwd: string, actionToRun: string, groupToRun: string): Promise<void> {
	const cnfPath = `${cwd}/.git-local-devops.yml`;
	const dotenvPath = `${cwd}/.git-local-devops-env`;

	let fileContent;

	if (await fs.pathExists(dotenvPath)) {
		const envCnf = dotenv.parse(await fs.readFile(dotenvPath)); // will return an object
		assert(envCnf['REMOTE_GIT_PROJECT'], `REMOTE_GIT_PROJECT isn't defined in ${dotenvPath}`);
		assert(envCnf['REMOTE_GIT_PROJECT_FILE'], `REMOTE_GIT_PROJECT_FILE isn't defined in ${dotenvPath}`);
		await fs.ensureDir("/tmp/git-local-devops");
		await pcp.spawn(
			"git", ["archive", `--remote=${envCnf['REMOTE_GIT_PROJECT']}`, "master", envCnf['REMOTE_GIT_PROJECT_FILE'], "|", "tar", "-xC", "/tmp/git-local-devops/"],
			{shell: "bash", cwd, env: process.env, encoding: "utf8"},
		);
		fileContent = await fs.readFile(`/tmp/git-local-devops/${envCnf['REMOTE_GIT_PROJECT_FILE']}`, "utf8");
	} else if (await fs.pathExists(cnfPath)) {
			fileContent = await fs.readFile(cnfPath, "utf8");
	} else {
		assert(process.env['GIT_LOCAL_DEVOPS_DEFAULT_CWD'], `No config was found in the current location and GIT_LOCAL_DEVOPS_DEFAULT_CWD isn't defined`);
		console.log(chalk`{yellow No config was found in the current location, using GIT_LOCAL_DEVOPS_DEFAULT_CWD:} {cyan ${process.env['GIT_LOCAL_DEVOPS_DEFAULT_CWD']}}`);
		return start(process.env['GIT_LOCAL_DEVOPS_DEFAULT_CWD'], actionToRun, groupToRun);
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