const fs = require("fs-extra");
const yaml = require("js-yaml");
const {runScripts} = require("./run_scripts");
const {gitOperations} = require("./git_operations");
const assert = require("assert");
const {startup} = require("./startup");
const chalk = require("chalk");

async function start(cwd, scriptToRun, domainToRun) {
	process.on("uncaughtException", (e) => {
		if (e instanceof assert.AssertionError) {
			console.error(chalk`{red ${e.message}}`);
		} else if (e.message.startsWith("Process exited")) {
			console.error(chalk`{red ${`${e["stderr"]}`.trim()}}`);
		} else {
			throw e;
		}
	});

	const cnfPath = `${cwd}/git-local-devops.yml`;
	assert(await fs.pathExists(cnfPath), `${cwd} doesn't contain an git-local-devops.yml file`);

	const fileContent = await fs.readFile(`${cwd}/git-local-devops.yml`, "utf8");
	const cnf = yaml.load(fileContent);

	await startup(cnf["startup"] ?? []);

	// General fail-early assertions on projects objects
	for (const projectObj of cnf["projects"]) {
		const remote = projectObj["remote"];
		const defaultBranch = projectObj["default_branch"];
		assert(defaultBranch != null, `default_branch not set for ${remote}`);
	}

	const gitOperationsPromises = [];
	for (const projectObj of cnf["projects"]) {
		gitOperationsPromises.push(gitOperations(cwd, projectObj));
	}
	await Promise.all(gitOperationsPromises);

	for (const projectObj of cnf["projects"]) {
		await runScripts(cwd, projectObj, scriptToRun, domainToRun);
	}
}

module.exports = {start};
