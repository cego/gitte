const fs = require("fs-extra");
const cp = require("promisify-child-process");
const chalk = require("chalk");
const assert = require("assert");
const to = require("await-to-js").default;

async function hasLocalChanges(dir) {
	const res = await cp.spawn("git", ["status", "--porcelain"], {cwd: dir, encoding: "utf8"});
	return `${res.stdout}`.trim().length !== 0;
}

async function gitOperations(cwd, projectObj) {
	const remote = projectObj["remote"];
	const defaultBranch = projectObj["default_branch"];
	assert(defaultBranch != null, `default_branch not set for ${remote}`);
	const dir = `${cwd}/${remote.replace(/.*?:/, "").replace(".git", "")}`;

	let err, res, currentBranch = null;
	[err, res] = await to(cp.spawn("git", ["rev-parse", "--abbrev-ref", "HEAD"], {cwd: dir, encoding: "utf8"}));
	if (!err) currentBranch = `${res.stdout}`.trim();

	if (!await fs.pathExists(`${dir}`)) {
		await cp.spawn("git", ["clone", remote, `${dir}`], {encoding: "utf8"});
		console.log(chalk`Cloned {gray ${remote}} to {cyan ${dir}}`);
	} else if (await hasLocalChanges(dir)) {
		console.log(chalk`Local changes found, no git operations will be applied in {cyan ${dir}}`);
	} else if (currentBranch === defaultBranch) {
		[err, res] = await to(cp.spawn("git", ["pull"], {cwd: dir, encoding: "utf8"}));
		if (err) throw err;
		const msg = `${res.stdout}`.trim();
		if (msg === "Already up to date.") {
			console.log(chalk`Already up to date {cyan ${dir}}`);
		} else {
			console.log(chalk`Pulled {magenta origin/${currentBranch}} in {cyan ${dir}}`);
		}
	} else {
		[err, _] = await to(cp.spawn("git", ["rebase", `origin/${defaultBranch}`], {cwd: dir, encoding: "utf8"}));
		if (!err) {
			console.log(chalk`Rebased {yellow ${currentBranch} on top of {magenta origin/${defaultBranch}} in {cyan ${dir}}`);
			return;
		}
		await cp.spawn("git", ["rebase", `--abort`], {cwd: dir, encoding: "utf8"});

		[err, _] = await to(cp.spawn("git", ["merge", `origin/${defaultBranch}`], {cwd: dir, encoding: "utf8"}));
		if (!err) {
			console.log(chalk`Merged {yellow {magenta origin/${defaultBranch}} with ${currentBranch} in {cyan ${dir}}`);
			return;
		}
		await cp.spawn("git", ["merge", `--abort`], {cwd: dir, encoding: "utf8"});
	}
}

async function runScripts(cwd, projectObj, scriptToRun, domainToRun) {
	const remote = projectObj["remote"];
	const scriptsObj = projectObj["scripts"];
	const defaultBranch = projectObj["default_branch"];
	assert(defaultBranch != null, `default_branch not set for ${remote}`);
	const dir = `${cwd}/${remote.replace(/.*?:/, "").replace(".git", "")}`;

	for (const [scriptName, domainsObj] of Object.entries(scriptsObj)) {
		// noinspection JSCheckFunctionSignatures
		for (const [domain, argv] of Object.entries(domainsObj)) {
			if (scriptName !== scriptToRun || domain !== domainToRun) continue;
			console.log(chalk`Executing {blue ${argv.join(" ")}} in {cyan ${dir}}`);
			await cp.spawn(argv[0], argv.slice(1), {cwd: dir, env: process.env, encoding: "utf8"});
		}
	}


}

module.exports = {runScripts, gitOperations};
