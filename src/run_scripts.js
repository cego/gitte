const {getProjectDirFromRemote} = require("./project");
const cp = require("promisify-child-process");
const chalk = require("chalk");
const {default: to} = require("await-to-js");

async function runScripts(cwd, projectObj, scriptToRun, domainToRun) {
	const remote = projectObj["remote"];
	const scriptsObj = projectObj["scripts"];
	const dir = getProjectDirFromRemote(cwd, remote);
	let err;

	for (const [scriptName, domainsObj] of Object.entries(scriptsObj)) {
		for (const [domain, argv] of Object.entries(domainsObj)) {
			if (scriptName !== scriptToRun || domain !== domainToRun) continue;
			console.log(chalk`Executing {blue ${argv.join(" ")}} in {cyan ${dir}}`);
			[err] = await to(cp.spawn(argv[0], argv.slice(1), {cwd: dir, env: process.env, encoding: "utf8"}));
			if (err) {
				console.error(chalk`${scriptToRun} for ${domainToRun} failed, goto {cyan ${dir}} and run {blue ${argv.join(" ")}} manually`);
			}
		}
	}
}

module.exports = {runScripts};
