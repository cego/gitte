const {getProjectDirFromRemote} = require("../src/project");
const {runScripts} = require("../src/run_scripts");
const {gitOperations} = require("../src/git_operations");
const cp = require("promisify-child-process");
const fs = require("fs-extra");
const chalk = require("chalk");

let cwd, projectObj, readFileSpy, pathExistsSpy, cpSpawnSpy;
beforeEach(() => {
	cp.spawn = jest.fn();
	console.log = jest.fn();

	cpSpawnSpy = jest.spyOn(cp, "spawn").mockImplementation(() => {
		return new Promise((resolve) => {
			resolve({stdout: "Mocked Stdout"});
		});
	});
	readFileSpy = jest.spyOn(fs, "readFile").mockImplementation(() => "---");
	pathExistsSpy = jest.spyOn(fs, "pathExists").mockImplementation(() => true);

	cwd = "/home/user/git-local-devops";
	projectObj = {
		default_branch: "main",
		remote: "git@gitlab.com:firecow/example.git",
		scripts: {
			start: {
				"firecow.dk": ["docker-compose", "up"],
				"example.com": ["scp", "user@example.com", "sh", "-c", "service", "webserver", "start"],
			},
			down: {
				"firecow.dk": ["docker-compose", "down"],
				"example.com": ["scp", "user@example.com", "sh", "-c", "service", "webserver", "stop"],
			},
		},
	};
});
afterEach(() => {
	jest.clearAllMocks();
});


describe("Project dir from remote", () => {

	test("valid ssh remote", () => {
		const dir = getProjectDirFromRemote(cwd, "git@gitlab.com:firecow/example.git");
		expect(dir).toEqual(`${cwd}/firecow/example`);
	});

	test("valid ssh remote with cwd ending in slash", () => {
		const dir = getProjectDirFromRemote(`${cwd}/`, "git@gitlab.com:firecow/example.git");
		expect(dir).toEqual(`${cwd}/firecow/example`);
	});

	test("invalid remote", () => {
		expect(() => {
			getProjectDirFromRemote(cwd, "git@gitlab.coinvalidirecow/example.git");
		}).toThrowError("git@gitlab.coinvalidirecow/example.git is not a valid project remote. Use git@gitlab.com:example/firecow.git syntax");
	});

});

describe("Run scripts", () => {

	test("start firecow.dk", async () => {
		await runScripts(cwd, projectObj, "start", "firecow.dk");
		expect(console.log).toHaveBeenCalledWith(chalk`Executing {blue docker-compose up} in {cyan /home/user/git-local-devops/firecow/example}`);
	});

});

describe("Git Operations", () => {

	test("existing project directory", async () => {
		pathExistsSpy = jest.spyOn(fs, "pathExists").mockResolvedValue(true);
		await gitOperations(cwd, projectObj);
	});

	test("non existing project directory", async () => {
		pathExistsSpy = jest.spyOn(fs, "pathExists").mockResolvedValue(false);
		await gitOperations(cwd, projectObj);
	});

});
