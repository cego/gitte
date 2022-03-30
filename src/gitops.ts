import { getProjectDirFromRemote } from "./project";
import to from "await-to-js";
import fs from "fs-extra";
import chalk from "chalk";
import { Config, Project } from "./types/config";
import * as pcp from "promisify-child-process";
import { ToChildProcessOutput } from "./types/utils";
import { printLogs } from "./utils";

async function hasLocalChanges(dir: string) {
	const res = await pcp.spawn("git", ["status", "--porcelain"], {
		cwd: dir,
		encoding: "utf8",
	});
	return `${res.stdout}`.trim().length !== 0;
}

async function fetch(dir: string) {
	await pcp.spawn("git", ["fetch"], { cwd: dir, encoding: "utf8" });
}

async function pull(dir: string, currentBranch: string, log: (arg: any) => void = console.log) {
	const [err, res]: ToChildProcessOutput = await to(pcp.spawn("git", ["pull"], { cwd: dir, encoding: "utf8" }));

	if (err || !res) {
		if (`${err?.stderr}`.trim().startsWith("There is no tracking information for the current branch")) {
			log(chalk`{yellow ${currentBranch}} doesn't have a remote origin {cyan ${dir}}`);
		} else {
			log(chalk`{yellow ${currentBranch}} {red conflicts} with {magenta origin/${currentBranch}} {cyan ${dir}}`);
		}
		return false;
	}

	const msg = `${res.stdout}`.trim();
	if (msg === "Already up to date.") {
		log(chalk`{yellow ${currentBranch}} is up to date in {cyan ${dir}}`);
	} else {
		log(chalk`{yellow ${currentBranch}} pulled changes from {magenta origin/${currentBranch}} in {cyan ${dir}}`);
	}
	return true;
}

async function rebase(
	dir: string,
	currentBranch: string,
	defaultBranch: string,
	log: (arg: any) => void = console.log,
) {
	const [err, res]: ToChildProcessOutput = await to(
		pcp.spawn("git", ["rebase", `origin/${defaultBranch}`], {
			cwd: dir,
			encoding: "utf8",
		}),
	);

	if (err || !res) {
		await pcp.spawn("git", ["rebase", "--abort"], {
			cwd: dir,
			encoding: "utf8",
		});
		return false;
	}

	if (`${res.stdout}`.trim() === `Current branch ${currentBranch} is up to date.`) {
		log(chalk`{yellow ${currentBranch}} is already on {magenta origin/${defaultBranch}} in {cyan ${dir}}`);
	} else {
		log(chalk`{yellow ${currentBranch}} was rebased on {magenta origin/${defaultBranch}} in {cyan ${dir}}`);
	}
	return true;
}

async function merge(dir: string, currentBranch: string, defaultBranch: string, log: (arg: any) => void = console.log) {
	const [err, res]: ToChildProcessOutput = await to(
		pcp.spawn("git", ["merge", `origin/${defaultBranch}`], {
			cwd: dir,
			encoding: "utf8",
		}),
	);
	if (!err && res) {
		log(chalk`{yellow ${currentBranch}} was merged with {magenta origin/${defaultBranch}} in {cyan ${dir}}`);
		return;
	}
	await pcp.spawn("git", ["merge", "--abort"], { cwd: dir, encoding: "utf8" });
	log(chalk`{yellow ${currentBranch}} merge with {magenta origin/${defaultBranch}} {red failed} in {cyan ${dir}}`);
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

	const [err, res]: ToChildProcessOutput = await to(
		pcp.spawn("git", ["branch", "--show-current"], {
			cwd: dir,
			encoding: "utf8",
		}),
	);

	if (err || !res) {
		log(chalk`{yellow ${remote}} {red failed} in {cyan ${dir}} ${err}`);
		log(res);
		return logs;
	}

	const currentBranch = `${res.stdout}`.trim();

	if (await hasLocalChanges(dir)) {
		log(chalk`{yellow ${currentBranch}} has local changes in {cyan ${dir}}`);
	} else if (currentBranch === defaultBranch) {
		await fetch(dir);
		await pull(dir, currentBranch, log);
	} else {
		await fetch(dir);
		if (!(await pull(dir, currentBranch, log))) return logs;
		if (!(await rebase(dir, currentBranch, defaultBranch, log))) {
			await merge(dir, currentBranch, defaultBranch, log);
		}
	}
	return logs;
}

export async function fromConfig(cwd: string, cnf: Config) {
	const gitOperationsPromises = Object.values(cnf.projects).map((project) => gitops(cwd, project));
	const logs = await Promise.all(gitOperationsPromises.map((p) => p.catch((e) => e)));
	printLogs(Object.keys(cnf.projects), logs);
}
