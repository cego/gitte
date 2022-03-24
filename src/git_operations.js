const {getProjectDirFromRemote} = require("./project");
const cp = require("promisify-child-process");
const {default: to} = require("await-to-js");
const fs = require("fs-extra");
const chalk = require("chalk");

async function hasLocalChanges(dir) {
	const res = await cp.spawn("git", ["status", "--porcelain"], {cwd: dir, encoding: "utf8"});
	return `${res.stdout}`.trim().length !== 0;
}

async function fetch(dir) {
	await cp.spawn("git", ["fetch"], {cwd: dir, encoding: "utf8"});
}

async function pull(dir, currentBranch) {
	let err, res;
	[err, res] = await to(cp.spawn("git", ["pull"], {cwd: dir, encoding: "utf8"}));
	if (err) throw err;
	const msg = `${res.stdout}`.trim();
	if (msg === "Already up to date.") {
		console.log(chalk`{yellow ${currentBranch}} is up to date in {cyan ${dir}}`);
	} else {
		console.log(chalk`{yellow ${currentBranch}} pulled changes from {magenta origin/${currentBranch}} in {cyan ${dir}}`);
	}
}

async function rebase(dir, currentBranch, defaultBranch) {
	let err, res;
	[err, res] = await to(cp.spawn("git", ["rebase", `origin/${defaultBranch}`], {cwd: dir, encoding: "utf8"}));
	if (err) {
		await cp.spawn("git", ["rebase", `--abort`], {cwd: dir, encoding: "utf8"});
		return false;
	}

	if (`${res.stdout}`.trim() === "Current branch test is up to date.") {
		console.log(chalk`Current branch {yellow ${currentBranch}} is already on top of {magenta origin/${defaultBranch}} in {cyan ${dir}}`);
	} else {
		console.log(chalk`Rebased {yellow ${currentBranch}} on top of {magenta origin/${defaultBranch}} in {cyan ${dir}}`);
	}


	return true;
}

async function merge(dir, currentBranch, defaultBranch) {
	let err;
	[err] = await to(cp.spawn("git", ["merge", `origin/${defaultBranch}`], {cwd: dir, encoding: "utf8"}));
	if (!err) {
		console.log(chalk`Merged {magenta origin/${defaultBranch}} with {yellow ${currentBranch}} in {cyan ${dir}}`);
		return;
	}
	await cp.spawn("git", ["merge", `--abort`], {cwd: dir, encoding: "utf8"});
	console.log(chalk`Merged failed in {cyan ${dir}}`);
}

async function gitOperations(cwd, projectObj) {
	const remote = projectObj["remote"];
	const defaultBranch = projectObj["default_branch"];
	const dir = getProjectDirFromRemote(cwd, remote);
	let err, res, currentBranch = null;

	[err, res] = await to(cp.spawn("git", ["rev-parse", "--abbrev-ref", "HEAD"], {cwd: dir, encoding: "utf8"}));
	if (!err) currentBranch = `${res.stdout}`.trim();

	if (!await fs.pathExists(`${dir}`)) {
		await cp.spawn("git", ["clone", remote, `${dir}`], {encoding: "utf8"});
		console.log(chalk`Cloned {gray ${remote}} to {cyan ${dir}}`);
	} else if (await hasLocalChanges(dir)) {
		console.log(chalk`Local changes found, no git operations will be applied in {cyan ${dir}}`);
	} else if (currentBranch === defaultBranch) {
		await fetch(dir);
		await pull(dir, currentBranch);
	} else {
		await fetch(dir);
		await pull(dir, currentBranch);
		if (!await rebase(dir, currentBranch, defaultBranch)) {
			await merge(dir, currentBranch, defaultBranch);
		}
	}
}

module.exports = {gitOperations};
