import { getProjectDirFromRemote } from "./project";
import to from "await-to-js";
import fs from "fs-extra";
import chalk from "chalk";
import { Project } from "./types/config";
import { Utils } from "./utils";
import { ChildProcessError, ChildProcessResult } from "./types/utils";

async function hasLocalChanges(dir: string) {
	const res = await Utils.spawn("git", ["status", "--porcelain"], { cwd: dir });
	return `${res.stdout}`.trim().length !== 0;
}

async function fetch(dir: string) {
	await Utils.spawn("git", ["fetch"], { cwd: dir });
}

async function pull(dir: string, currentBranch: string) {
	let err: ChildProcessError | null, res: ChildProcessResult | undefined;
	[err, res] = await to(Utils.spawn("git", ["pull"], { cwd: dir }));

	if (err || !res) {
		if (`${err?.stderr}`.trim().startsWith("There is no tracking information for the current branch")) {
			console.log(chalk`{yellow ${currentBranch}} doesn't have a remote origin {cyan ${dir}}`);
		} else {
			console.log(chalk`{yellow ${currentBranch}} {red conflicts} with {magenta origin/${currentBranch}} {cyan ${dir}}`);
		}
		return false;
	}


	const msg = `${res.stdout}`.trim();
	if (msg === "Already up to date.") {
		console.log(chalk`{yellow ${currentBranch}} is up to date in {cyan ${dir}}`);
	} else {
		console.log(chalk`{yellow ${currentBranch}} pulled changes from {magenta origin/${currentBranch}} in {cyan ${dir}}`);
	}
	return true;
}

async function rebase(dir: string, currentBranch: string, defaultBranch: string) {
	let err: ChildProcessError | null, res: ChildProcessResult | undefined;
	[err, res] = await to(Utils.spawn("git", ["rebase", `origin/${defaultBranch}`], { cwd: dir }));

	if (err || !res) {
		await Utils.spawn("git", ["rebase", "--abort"], { cwd: dir });
		return false;
	}

	if (`${res.stdout}`.trim() === "Current branch test is up to date.") {
		console.log(chalk`{yellow ${currentBranch}} is already on {magenta origin/${defaultBranch}} in {cyan ${dir}}`);
	} else {
		console.log(chalk`{yellow ${currentBranch}} was rebased on {magenta origin/${defaultBranch}} in {cyan ${dir}}`);
	}
	return true;
}

async function merge(dir: string, currentBranch: string, defaultBranch: string) {
	let err: ChildProcessError | null, res: ChildProcessResult | undefined;
	[err, res] = await to(Utils.spawn("git", ["merge", `origin/${defaultBranch}`], { cwd: dir }));
	if (!err && res) {
		console.log(chalk`{yellow ${currentBranch}} was merged with {magenta origin/${defaultBranch}} in {cyan ${dir}}`);
		return;
	}
	await Utils.spawn("git", ["merge", "--abort"], { cwd: dir });
	console.log(chalk`{yellow ${currentBranch}} merge with {magenta origin/${defaultBranch}} {red failed} in {cyan ${dir}}`);
}

export async function gitOperations(cwd: string, projectObj: Project) {
	const remote = projectObj["remote"];
	const defaultBranch = projectObj["default_branch"];
	const dir = getProjectDirFromRemote(cwd, remote);

	if (!await fs.pathExists(`${dir}`)) {
		await Utils.spawn("git", ["clone", remote, dir], { cwd });
		console.log(chalk`{gray ${remote}} was cloned to {cyan ${dir}}`);
		return;
	}

	let err: ChildProcessError | null, res: ChildProcessResult | undefined;
	[err, res] = await to(Utils.spawn("git", ["branch", "--show-current"], { cwd: dir }));

	if (err || !res) {
		console.log(chalk`{yellow ${remote}} {red failed} in {cyan ${dir}} ${err}`);
		console.log(res);
		return;
	}

	const currentBranch = `${res.stdout}`.trim();
	
	if (await hasLocalChanges(dir)) {
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