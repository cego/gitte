const fs = require("fs-extra");
const yaml = require("js-yaml");
const {runScripts} = require("./run_scripts");
const {gitOperations} = require("./git_operations");
const assert = require("assert");
const {startup} = require("./startup");
const cp = require("promisify-child-process");
const dotenv = require("dotenv");

async function start(cwd, scriptToRun, domainToRun) {
	const cnfPath = `${cwd}/.git-local-devops.yml`;
	const dotenvPath = `${cwd}/.git-local-devops-env`;
	const prioRange = [0, 1000];

	let fileContent;

	if (await fs.pathExists(dotenvPath)) {
		const envCnf = dotenv.parse(await fs.readFile(dotenvPath)); // will return an object
		assert(envCnf.REMOTE_GIT_PROJECT, `REMOTE_GIT_PROJECT isn't defined in ${dotenvPath}`);
		assert(envCnf.REMOTE_GIT_PROJECT_FILE, `REMOTE_GIT_PROJECT_FILE isn't defined in ${dotenvPath}`);
		await fs.ensureDir("/tmp/git-local-devops");
		await cp.spawn(
			`git archive --remote=${envCnf.REMOTE_GIT_PROJECT} master ${envCnf.REMOTE_GIT_PROJECT_FILE} | tar -xC /tmp/git-local-devops/`,
			{shell: "bash", cwd, env: process.env, encoding: "utf8"},
		);
		fileContent = await fs.readFile(`/tmp/git-local-devops/${envCnf.REMOTE_GIT_PROJECT_FILE}`, "utf8");
	} else {
		assert(await fs.pathExists(cnfPath), `${cwd} doesn't contain an .git-local-devops.yml file`);
		fileContent = await fs.readFile(`${cwd}/.git-local-devops.yml`, "utf8");
	}

	const cnf = yaml.load(fileContent);

	assert(cnf["startup"], `config must contain startup map`);
	await startup(cnf["startup"]);

	// General fail-early assertions on projects objects
	for (const projectObj of Object.values(cnf["projects"])) {
		const remote = projectObj["remote"];
		const defaultBranch = projectObj["default_branch"];
		assert(defaultBranch != null, `default_branch not set for ${remote}`);
		const priority = projectObj["priority"];
		assert(priority != null, `priority not set for ${remote}`);
		assert(priority < prioRange[1], `priority must be below ${prioRange[1]}`);
		assert(priority >= prioRange[0], `priority must be above or equal ${prioRange[0]}`);
	}

	const gitOperationsPromises = [];
	for (const projectObj of Object.values(cnf["projects"])) {
		gitOperationsPromises.push(gitOperations(cwd, projectObj));
	}
	await Promise.all(gitOperationsPromises);

	for (let i = prioRange[0]; i < prioRange[1]; i++) {
		const runScriptsPromises = [];
		for (const projectObj of Object.values(cnf["projects"]).filter((p) => p.priority === i)) {
			runScriptsPromises.push(runScripts(cwd, projectObj, scriptToRun, domainToRun));
		}
		await Promise.all(runScriptsPromises);
	}
}

module.exports = {start};
