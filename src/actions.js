const {getProjectDirFromRemote} = require("./project");
const cp = require("promisify-child-process");
const chalk = require("chalk");
const {default: to} = require("await-to-js");

async function runActions(cwd, projectObj, currentPrio, actionToRun, groupToRun) {
	const remote = projectObj["remote"];
	const actionsObj = projectObj["actions"];
	const dir = getProjectDirFromRemote(cwd, remote);
	let err;

	for (const [actionName, actionObj] of Object.entries(actionsObj)) {
		if (actionName !== actionToRun) continue;

		for (const [groupName, cmd] of Object.entries(actionObj["groups"])) {
			const priority = actionObj["priority"] ?? projectObj["priority"] ?? 0;
			if (currentPrio !== priority) continue;
			if (groupName !== groupToRun) continue;
			console.log(chalk`{blue ${cmd.join(" ")}} is running in {cyan ${dir}}`);
			[err] = await to(cp.spawn(cmd[0], cmd.slice(1), {cwd: dir, env: process.env, encoding: "utf8"}));
			if (err) {
				console.error(chalk`"${actionToRun}" "${groupToRun}" {red failed}, goto {cyan ${dir}} and run {blue ${cmd.join(" ")}} manually`);
			}
		}
	}
}

module.exports = {runActions};
