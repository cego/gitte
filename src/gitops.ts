import { getProjectDirFromRemote } from "./project";
import to from "await-to-js";
import fs from "fs-extra";
import chalk from "chalk";
import { Config, Project } from "./types/config";
import * as pcp from "promisify-child-process";
import { ErrorWithHint, ToChildProcessOutput } from "./types/utils";
import { printHeader, printLogs } from "./utils";
import { applyPromiseToEntriesWithProgressBar } from "./progress";

type LogFn = (arg: string | ErrorWithHint) => void;

async function hasLocalChanges(dir: string): Promise<boolean> {
	const [err, res]: ToChildProcessOutput = await to(
		pcp.spawn("git", ["status", "--porcelain"], { cwd: dir, encoding: "utf8" }),
	);
	if (err || !res) {
		throw new ErrorWithHint(chalk`{yellow ${dir}} {red failed} to check for local changes ${err?.stderr}`);
	}
	return `${res.stdout}`.trim().length !== 0;
}

async function pull(dir: string, currentBranch: string, log: LogFn) {
	const pcpPromise = pcp.spawn("git", ["pull", "--ff-only"], { cwd: dir, encoding: "utf8" });
	const [err, res]: ToChildProcessOutput = await to(pcpPromise);

	if (err || !res) {
		if (`${err?.stderr}`.trim().startsWith("There is no tracking information for the current branch")) {
			log(chalk`{cyan ${currentBranch}} {red doesn't have a remote origin} {cyan ${dir}}`);
		} else {
			log(chalk`{cyan ${currentBranch}} {red conflicts} with {magenta origin/${currentBranch}} {cyan ${dir}}`);
		}
		return false;
	}

	const msg = `${res.stdout}`.trim();
	if (msg === "Already up to date.") {
		log(chalk`{cyan ${currentBranch}} is up to date with {magenta origin/${currentBranch}} in {cyan ${dir}}`);
	} else {
		log(chalk`{cyan ${currentBranch}} pulled changes from {magenta origin/${currentBranch}} in {cyan ${dir}}`);
	}
	return true;
}

async function mergeError(dir: string, currentB: string, defaultB: string, log: LogFn) {
	log(chalk`{yellow ${currentB}} merge with {magenta origin/${defaultB}} {red failed} in {cyan ${dir}}`);

	const pcpPromise = pcp.spawn("git", ["merge", "--abort"], { cwd: dir, encoding: "utf8" });
	const [err] = await to(pcpPromise);
	if (err) {
		log(chalk`{yellow ${currentB}} merge --abort also {red failed} in {cyan ${dir}}`);
	}
}

async function merge(dir: string, currentB: string, defaultBranch: string, log: LogFn) {
	const pcpPromise = pcp.spawn("git", ["merge", `origin/${defaultBranch}`], { cwd: dir, encoding: "utf8" });
	const [err, res] = await to(pcpPromise);

	if (err || !res) {
		return mergeError(dir, currentB, defaultBranch, log);
	}

	const msg = `${res.stdout}`.trim();
	if (msg === "Already up to date.") {
		log(chalk`{cyan ${currentB}} is up to date with {magenta origin/${defaultBranch}} in {cyan ${dir}}`);
	} else {
		log(chalk`{yellow {cyan ${currentB}} was merged with {magenta origin/${defaultBranch}} in {cyan ${dir}}}`);
	}
}

async function clone(cwd: string, remote: string, dir: string, log: LogFn) {
	const pcpPromise = pcp.spawn("git", ["clone", remote, dir], { cwd, encoding: "utf8" });
	const [err, res]: ToChildProcessOutput = await to(pcpPromise);
	if (err || !res) {
		if (err?.stderr?.includes("Permission denied")) {
			const errorMessage = `Permission denied to clone ${remote}`;
			log(new ErrorWithHint(errorMessage, errorMessage));
			return;
		}
		log(new ErrorWithHint(chalk`{yellow ${remote}} {red failed} in {cyan ${dir}} \n${err?.stderr}`));
		return;
	}
	log(chalk`{gray ${remote}} was cloned to {cyan ${dir}}`);
}

export async function gitops(cwd: string, projectObj: Project): Promise<(string | ErrorWithHint)[]> {
	const logs: (string | ErrorWithHint)[] = [];
	const log = (arg: string | ErrorWithHint) => {
		logs.push(arg);
	};
	const remote = projectObj["remote"];
	const defaultBranch = projectObj["default_branch"];
	const dir = getProjectDirFromRemote(cwd, remote);

	if (!(await fs.pathExists(`${dir}`))) {
		await clone(cwd, remote, dir, log);
		return logs;
	}

	const pcpPromise = pcp.spawn("git", ["branch", "--show-current"], { cwd: dir, encoding: "utf8" });
	const [err, res]: ToChildProcessOutput = await to(pcpPromise);

	if (err || !res) {
		log(new ErrorWithHint(chalk`{yellow ${remote}} {red failed} in {cyan ${dir}} ${err}`));
		return logs;
	}

	const currentBranch = `${res.stdout}`.trim();

	let localChanges: boolean;
	try {
		localChanges = await hasLocalChanges(dir);
	} catch (err) {
		if (err instanceof ErrorWithHint) {
			log(err);
			return logs;
		}
		throw err;
	}

	if (localChanges) {
		log(chalk`{yellow ${currentBranch}} has local changes in {cyan ${dir}}`);
	} else if (currentBranch === defaultBranch) {
		await pull(dir, currentBranch, log);
	} else {
		if (await pull(dir, currentBranch, log)) {
			await merge(dir, currentBranch, defaultBranch, log);
		}
	}
	return logs;
}

export async function fromConfig(cnf: Config) {
	printHeader("Git Operations");
	const fn = (arg: Project) => gitops(cnf.cwd, arg);
	const result = await applyPromiseToEntriesWithProgressBar("git-operations", Object.entries(cnf.projects), fn);
	console.log();
	printLogs(Object.keys(cnf.projects), result);
}
