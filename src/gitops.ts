import { getProjectDirFromRemote } from "./project";
import to from "await-to-js";
import fs from "fs-extra";
import chalk from "chalk";
import { Config, Project } from "./types/config";
import * as pcp from "promisify-child-process";
import { ToChildProcessOutput } from "./types/utils";
import { printHeader, printLogs } from "./utils";
import { applyPromiseToEntriesWithProgressBar } from "./progress";

async function hasLocalChanges(dir: string) {
	const res = await pcp.spawn("git", ["status", "--porcelain"], { cwd: dir, encoding: "utf8" });
	return `${res.stdout}`.trim().length !== 0;
}

async function pull(dir: string, currentBranch: string, log: (arg: any) => void) {
	const pcpPromise = pcp.spawn("git", ["pull", "--ff-only"], { cwd: dir, encoding: "utf8" });
	const [err, res]: ToChildProcessOutput = await to(pcpPromise);

	if (err || !res) {
		if (`${err?.stderr}`.trim().startsWith("There is no tracking information for the current branch")) {
			log(chalk`{yellow ${currentBranch}} doesn't have a remote origin {cyan ${dir}}`);
		} else {
			log(chalk`{yellow ${currentBranch}} {red conflicts} with {magenta origin/${currentBranch}} {cyan ${dir}}`);
		}
		return;
	}

	const msg = `${res.stdout}`.trim();
	if (msg === "Already up to date.") {
		log(chalk`{yellow ${currentBranch}} is up to date in {cyan ${dir}}`);
	} else {
		log(chalk`{yellow ${currentBranch}} pulled changes from {magenta origin/${currentBranch}} in {cyan ${dir}}`);
	}
}

async function merge(dir: string, currentBranch: string, defaultBranch: string, log: (arg: any) => void) {
	let m, pcpPromise, err;
	const mergeError = async function () {
		m = chalk`{yellow ${currentBranch}} merge with {magenta origin/${defaultBranch}} {red failed} in {cyan ${dir}}`;
		log(m);

		pcpPromise = pcp.spawn("git", ["merge", "--abort"], { cwd: dir, encoding: "utf8" });
		[err] = await to(pcpPromise);
		if (err) {
			m = chalk`{yellow ${currentBranch}} merge --abort also {red failed} in {cyan ${dir}}`;
			log(m);
		}
	};
	pcpPromise = pcp.spawn("git", ["merge", `origin/${defaultBranch}`], { cwd: dir, encoding: "utf8" });
	[err] = await to(pcpPromise);

	if (err) {
		return mergeError();
	}

	m = chalk`{yellow ${currentBranch}} was merged with {magenta origin/${defaultBranch}} in {cyan ${dir}}`;
	log(m);
}

export async function gitops(cwd: string, projectObj: Project): Promise<any[]> {
	const logs: any[] = [];
	const log = (arg: any) => {
		logs.push(arg);
	};
	const remote = projectObj["remote"];
	const defaultBranch = projectObj["default_branch"];
	const dir = getProjectDirFromRemote(cwd, remote);

	if (!(await fs.pathExists(`${dir}`))) {
		await pcp.spawn("git", ["clone", remote, dir], { cwd, encoding: "utf8" });
		log(chalk`{gray ${remote}} was cloned to {cyan ${dir}}`);
		return logs;
	}

	const pcpPromise = pcp.spawn("git", ["branch", "--show-current"], { cwd: dir, encoding: "utf8" });
	const [err, res]: ToChildProcessOutput = await to(pcpPromise);

	if (err || !res) {
		log(chalk`{yellow ${remote}} {red failed} in {cyan ${dir}} ${err}`);
		log(res);
		return logs;
	}

	const currentBranch = `${res.stdout}`.trim();

	if (await hasLocalChanges(dir)) {
		log(chalk`{yellow ${currentBranch}} has local changes in {cyan ${dir}}`);
	} else if (currentBranch === defaultBranch) {
		await pull(dir, currentBranch, log);
	} else {
		await pull(dir, currentBranch, log);
		await merge(dir, currentBranch, defaultBranch, log);
	}
	return logs;
}

export async function fromConfig(cwd: string, cnf: Config) {
	printHeader("Git Operations");
	const fn = (arg: Project) => gitops(cwd, arg);
	const result = await applyPromiseToEntriesWithProgressBar("git-operations", Object.entries(cnf.projects), fn);
	console.log();
	printLogs(Object.keys(cnf.projects), result);
}
