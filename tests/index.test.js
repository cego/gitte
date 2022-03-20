const {getProjectDirFromRemote} = require("../src/project");
const {runScripts} = require("../src/run_scripts");
const {gitOperations} = require("../src/git_operations");
const cp = require("promisify-child-process");
const fs = require("fs-extra");
const chalk = require("chalk");
const {when} = require("jest-when");

function mockHasNoChanges() {
	when(spawnSpy).calledWith("git", ["status", "--porcelain"], expect.objectContaining({})).mockResolvedValue({stdout: ""});
}

function mockCustomBranch() {
	when(spawnSpy).calledWith("git", ["rev-parse", "--abbrev-ref", "HEAD"], expect.objectContaining({})).mockResolvedValue({stdout: "custom"});
}

function mockRebaseFailed() {
	when(spawnSpy).calledWith("git", ["rebase", `origin/main`], expect.objectContaining({})).mockRejectedValue("Rebase wasn't possible");
}

function mockMergeFailed() {
	when(spawnSpy).calledWith("git", ["merge", `origin/main`], expect.objectContaining({})).mockRejectedValue("Merge wasn't possible");
}

let cwd, projectObj, readFileSpy, pathExistsSpy, spawnSpy;
beforeEach(() => {
	cp.spawn = jest.fn();
	console.log = jest.fn();

	spawnSpy = jest.spyOn(cp, "spawn").mockImplementation(() => {
		return new Promise((resolve) => {
			resolve({stdout: "Mocked Stdout"});
		});
	});
	when(spawnSpy).calledWith(
		"git", ["rev-parse", "--abbrev-ref", "HEAD"], expect.objectContaining({}),
	).mockResolvedValue({stdout: "main"});
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

	test("Valid ssh remote", () => {
		const dir = getProjectDirFromRemote(cwd, "git@gitlab.com:firecow/example.git");
		expect(dir).toEqual(`${cwd}/firecow/example`);
	});

	test("Valid ssh remote with cwd ending in slash", () => {
		const dir = getProjectDirFromRemote(`${cwd}/`, "git@gitlab.com:firecow/example.git");
		expect(dir).toEqual(`${cwd}/firecow/example`);
	});

	test("Invalid remote", () => {
		expect(() => {
			getProjectDirFromRemote(cwd, "git@gitlab.coinvalidirecow/example.git");
		}).toThrowError("git@gitlab.coinvalidirecow/example.git is not a valid project remote. Use git@gitlab.com:example/firecow.git syntax");
	});

});

describe("Run scripts", () => {

	test("Start firecow.dk", async () => {
		await runScripts(cwd, projectObj, "start", "firecow.dk");
		expect(console.log).toHaveBeenCalledWith(
			chalk`Executing {blue docker-compose up} in {cyan /home/user/git-local-devops/firecow/example}`,
		);
	});

});

describe("Git Operations", () => {

	test("Changes found", async () => {
		pathExistsSpy = jest.spyOn(fs, "pathExists").mockResolvedValue(true);
		await gitOperations(cwd, projectObj);
		expect(console.log).toHaveBeenCalledWith(
			chalk`Local changes found, no git operations will be applied in {cyan ${cwd}/firecow/example}`,
		);
	});

	test("Cloning project", async () => {
		pathExistsSpy = jest.spyOn(fs, "pathExists").mockResolvedValue(false);
		await gitOperations(cwd, projectObj);
		expect(spawnSpy).toHaveBeenCalledWith(
			"git", ["clone", "git@gitlab.com:firecow/example.git", "/home/user/git-local-devops/firecow/example"],
			expect.objectContaining({}),
		);
	});

	describe("Default branch", () => {
		test("Already up to date", async () => {
			mockHasNoChanges();
			when(spawnSpy).calledWith(
				"git", ["pull"], expect.objectContaining({}),
			).mockResolvedValue({stdout: "Already up to date."});
			await gitOperations(cwd, projectObj);
			expect(console.log).toHaveBeenCalledWith(chalk`Already up to date {cyan ${cwd}/firecow/example}`);
			expect(spawnSpy).toHaveBeenCalledWith(
				"git", ["pull"],
				expect.objectContaining({}),
			);
		});

		test("Pulling latest changes", async () => {
			mockHasNoChanges();
			await gitOperations(cwd, projectObj);
			expect(console.log).toHaveBeenCalledWith(chalk`Pulled {magenta origin/main} in {cyan ${cwd}/firecow/example}`);
			expect(spawnSpy).toHaveBeenCalledWith(
				"git", ["pull"],
				expect.objectContaining({}),
			);
		});
	});

	describe("Custom branch", () => {

		test("Rebasing", async () => {
			mockHasNoChanges();
			mockCustomBranch();
			await gitOperations(cwd, projectObj);
			expect(console.log).toHaveBeenCalledWith(
				chalk`Rebased {yellow custom} on top of {magenta origin/main} in {cyan ${cwd}/firecow/example}`,
			);
			expect(spawnSpy).toHaveBeenCalledWith(
				"git", ["rebase", `origin/main`],
				expect.objectContaining({}),
			);
		});

		test("Rebase failed. Merging", async () => {
			mockHasNoChanges();
			mockCustomBranch();
			mockRebaseFailed();
			await gitOperations(cwd, projectObj);
			expect(console.log).toHaveBeenCalledWith(
				chalk`Merged {magenta origin/main} with {yellow custom} in {cyan ${cwd}/firecow/example}`,
			);
			expect(spawnSpy).toHaveBeenCalledWith(
				"git", ["rebase", `--abort`],
				expect.objectContaining({}),
			);
			expect(spawnSpy).toHaveBeenCalledWith(
				"git", ["merge", `origin/main`],
				expect.objectContaining({}),
			);
		});

		test("Merging failed", async () => {
			mockHasNoChanges();
			mockCustomBranch();
			mockRebaseFailed();
			mockMergeFailed();
			await gitOperations(cwd, projectObj);
			expect(console.log).toHaveBeenCalledWith(
				chalk`Merged failed in {cyan ${cwd}/firecow/example}`,
			);
			expect(spawnSpy).toHaveBeenCalledWith(
				"git", ["merge", `--abort`],
				expect.objectContaining({}),
			);
		});
	});
});
