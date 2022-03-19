const fs = require("fs-extra");
const cp = require("promisify-child-process");
const chalk = require("chalk");
const assert = require("assert");

async function doProject(cwd, projectObj, scriptToRun, domainToRun) {
	const remote = projectObj["remote"];
	const scriptsObj = projectObj["scripts"];
	const defaultBranch = projectObj["default_branch"];
	assert(defaultBranch != null, `default_branch not set for ${remote}`);
	const dir = `${cwd}/${remote.replace(/.*?:/, "").replace(".git", "")}`;
	if (!await fs.pathExists(`${dir}`)) {
		console.log(`Cloning ${remote} to ${dir}`);
		await cp.spawn("git", ["clone", remote, `${dir}`], {encoding: "utf8"});
	}

	// Pull the latest master, if on master

	// Rebase branch on top of master

	for (const [scriptName, domainsObj] of Object.entries(scriptsObj)) {
		// noinspection JSCheckFunctionSignatures
		for (const [domain, argv] of Object.entries(domainsObj)) {
			if (scriptName !== scriptToRun || domain !== domainToRun) continue;
			console.log(chalk`{blue ${argv.join(" ")}} is being executed in {cyan ${dir}}`);
			await cp.spawn(argv[0], argv.slice(1), {cwd: dir, env: process.env, encoding: "utf8"});
		}
	}


}

module.exports = {doProject};
