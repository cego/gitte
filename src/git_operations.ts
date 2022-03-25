import { getProjectDirFromRemote } from "./project";
import to from "await-to-js";
import fs from "fs-extra";
import chalk from "chalk";
import { Project } from "./validate_yaml";
import { asyncSpawn } from "./utils";


async function hasLocalChanges(dir: string) {
	const res = await asyncSpawn("git", ["status", "--porcelain"], {cwd: dir});
	return `${res.stdout}`.trim().length !== 0;
}

async function fetch(dir: string) {
	await asyncSpawn("git", ["fetch"], {cwd: dir});
}

async function pull(dir: string, currentBranch: string) {
	let stderr, stdout;
	[stderr, stdout] = await to(asyncSpawn("git", ["pull"], {cwd: dir}));

	if (stderr) {
		if (`${stderr}`.trim().startsWith("There is no tracking information for the current branch")) {
			console.log(chalk`{yellow ${currentBranch}} doesn't have a remote origin {cyan ${dir}}`);
		} else {
			console.log(chalk`{yellow ${currentBranch}} {red conflicts} with {magenta origin/${currentBranch}} {cyan ${dir}}`);
		}
		return false;
	}


	const msg = `${stdout}`.trim();
	if (msg === "Already up to date.") {
		console.log(chalk`{yellow ${currentBranch}} is up to date in {cyan ${dir}}`);
	} else {
		console.log(chalk`{yellow ${currentBranch}} pulled changes from {magenta origin/${currentBranch}} in {cyan ${dir}}`);
	}
	return true;
}

async function rebase(dir: string, currentBranch: string, defaultBranch: string) {
	let stderr, stdout;
	[stderr, stdout] = await to(asyncSpawn(`git rebase origin/${defaultBranch}`, {cwd: dir, encoding: "utf8"}));

	if (stderr) {
		await asyncSpawn("git rebase --abort", {cwd: dir, encoding: "utf8"});
		return false;
	}

	if (`${stdout}`.trim() === "Current branch test is up to date.") {
		console.log(chalk`{yellow ${currentBranch}} is already on {magenta origin/${defaultBranch}} in {cyan ${dir}}`);
	} else {
		console.log(chalk`{yellow ${currentBranch}} was rebased on {magenta origin/${defaultBranch}} in {cyan ${dir}}`);
	}
	return true;
}

async function merge(dir: string, currentBranch: string, defaultBranch: string) {
	let err;
	[err] = await to(asyncSpawn(`git merge origin/${defaultBranch}`, {cwd: dir, encoding: "utf8"}));
	if (!err) {
		console.log(chalk`{yellow ${currentBranch}} was merged with {magenta origin/${defaultBranch}} in {cyan ${dir}}`);
		return;
	}
	await asyncSpawn("git merge --abort", {cwd: dir, encoding: "utf8"});
	console.log(chalk`{yellow ${currentBranch}} merge with {magenta origin/${defaultBranch}} {red failed} in {cyan ${dir}}`);
}

export async function gitOperations(cwd: string, projectObj: Project) {
	const remote = projectObj["remote"];
	const defaultBranch = projectObj["default_branch"];
	const dir = getProjectDirFromRemote(cwd, remote);

	let stderr, stdout, currentBranch = null;

	[stderr, stdout] = await to(asyncSpawn("git rev-parse --agrev-ref HEAD", {cwd: dir, encoding: "utf8"}));

	if(stderr){
		console.log(chalk`{yellow ${remote}} {red failed} in {cyan ${dir}} ${stderr}`);
		return;
	}

	currentBranch = `${stdout}`.trim();

	if (!await fs.pathExists(`${dir}`)) {
		await asyncSpawn(`git clone ${remote} ${dir}`, {encoding: "utf8"});
		console.log(chalk`{gray ${remote}} was cloned to {cyan ${dir}}`);
	} else if (await hasLocalChanges(dir)) {
		console.log(chalk`{yellow ${currentBranch}} has local changes in {cyan ${dir}}`);
	} else if (currentBranch === defaultBranch) {
		await fetch(dir);
		await pull(dir, currentBranch);
	} else {
		await fetch(dir);
		if (!await pull(dir, currentBranch)) return;
		if (!await rebase(dir, currentBranch, defaultBranch)) {
			await merge(dir, currentBranch, defaultBranch);
		}
	}
}