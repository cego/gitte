const {getProjectDirFromRemote} = require("./project");

const cp = require("promisify-child-process");
const chalk = require("chalk");

async function runScripts(cwd, projectObj, scriptToRun, domainToRun) {
	const remote = projectObj["remote"];
	const scriptsObj = projectObj["scripts"];
	const dir = getProjectDirFromRemote(cwd, remote);

	for (const [scriptName, domainsObj] of Object.entries(scriptsObj)) {
		// noinspection JSCheckFunctionSignatures
		for (const [domain, argv] of Object.entries(domainsObj)) {
			if (scriptName !== scriptToRun || domain !== domainToRun) continue;
			console.log(chalk`Executing {blue ${argv.join(" ")}} in {cyan ${dir}}`);
			await cp.spawn(argv[0], argv.slice(1), {cwd: dir, env: process.env, encoding: "utf8"});
		}
	}
}

module.exports = {runScripts};
