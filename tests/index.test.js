const {getProjectDirFromRemote} = require("../src/project");
const {runScripts} = require("../src/run_scripts");
const {gitOperations} = require("../src/git_operations");
const cp = require("promisify-child-process");
const fs = require("fs-extra");
const chalk = require("chalk");
const {when} = require("jest-when");
const {start} = require("../src");
const yaml = require("js-yaml");
const {startup} = require("../src/startup");

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

let cwdStub, projectStub, startupStub, readFileSpy, pathExistsSpy, spawnSpy;
beforeEach(() => {
	cwdStub = "/home/user/git-local-devops";
	projectStub = {
		default_branch: "main",
		remote: "git@gitlab.com:firecow/example.git",
		priority: 0,
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
	startupStub = {
		world: {cmd: ["echo", "world"]},
		bashWorld: {shell: "bash", script: "echo world"},
	};
	readFileSpy = jest.spyOn(fs, "readFile").mockImplementation(() => {
		return `---\n${yaml.dump({projects: {example: projectStub}, startup: startupStub})}`;
	});
	cp.spawn = jest.fn();
	console.log = jest.fn();
	console.error = jest.fn();
	fs.pathExists = jest.fn();

	spawnSpy = jest.spyOn(cp, "spawn").mockImplementation(() => {
		return new Promise((resolve) => {
			resolve({stdout: "Mocked Stdout"});
		});
	});

	when(spawnSpy).calledWith(
		"git", ["rev-parse", "--abbrev-ref", "HEAD"], expect.objectContaining({}),
	).mockResolvedValue({stdout: "main"});
});

describe("Index (start)", () => {

	test("with default stubs", async () => {
		when(fs.pathExists).calledWith(`${cwdStub}/.git-local-devops-env`).mockResolvedValue(false);
		when(fs.pathExists).calledWith(`${cwdStub}/.git-local-devops.yml`).mockResolvedValue(true);
		await expect(start(cwdStub)).resolves.toBe();
	});

	test(".git-local-devops-env present", async () => {
		when(fs.pathExists).calledWith(`${cwdStub}/.git-local-devops-env`).mockResolvedValue(true);
		when(fs.pathExists).calledWith(`${cwdStub}/.git-local-devops.yml`).mockResolvedValue(true);
		when(readFileSpy).calledWith(`${cwdStub}/.git-local-devops-env`).mockImplementation(() => {
			return `REMOTE_GIT_PROJECT_FILE=".git-local-devops.yml"\nREMOTE_GIT_PROJECT="git@gitlab.com:firecow/example.git"\n`;
		});
		when(spawnSpy)
			.calledWith("git archive --remote=git@gitlab.com:firecow/example.git master .git-local-devops.yml | tar -xC /tmp/git-local-devops/")
			.mockResolvedValue(true);
		await expect(start(cwdStub)).resolves.toBe();
	});

	test("config file not found", async () => {
		when(fs.pathExists).calledWith(`${cwdStub}/.git-local-devops.yml`).mockResolvedValue(false);
		await expect(start("/home/user/completelyinvalidpath"))
			.rejects
			.toThrow("/home/user/completelyinvalidpath doesn't contain an .git-local-devops.yml file");
	});

});

describe("Startup checks", () => {

	test("failing argv", async () => {
		when(spawnSpy).calledWith("echo", ["hello"], expect.objectContaining({})).mockRejectedValue(new Error("WHAT"));
		await expect(startup([{cmd: ["echo", "hello"]}])).rejects.toThrow("WHAT");
	});

	test("failing shell", async () => {
		when(spawnSpy).calledWith("echo hello", expect.objectContaining({shell: "bash"})).mockRejectedValue(new Error("WHAT"));
		await expect(startup([{shell: "bash", script: "echo hello"}])).rejects.toThrow("WHAT");
	});

});

describe("Project dir from remote", () => {

	test("Valid ssh remote", () => {
		const dir = getProjectDirFromRemote(cwdStub, "git@gitlab.com:firecow/example.git");
		expect(dir).toEqual(`${cwdStub}/firecow-example`);
	});

	test("Valid ssh remote with cwd ending in slash", () => {
		const dir = getProjectDirFromRemote(`${cwdStub}/`, "git@gitlab.com:firecow/example.git");
		expect(dir).toEqual(`${cwdStub}/firecow-example`);
	});

	test("Invalid remote", () => {
		expect(() => {
			getProjectDirFromRemote(cwdStub, "git@gitlab.coinvalidirecow/example.git");
		}).toThrowError("git@gitlab.coinvalidirecow/example.git is not a valid project remote. Use git@gitlab.com:example/firecow.git syntax");
	});

});

describe("Run scripts", () => {

	test("Start firecow.dk", async () => {
		await runScripts(cwdStub, projectStub, "start", "firecow.dk");
		expect(console.log).toHaveBeenCalledWith(
			chalk`Executing {blue docker-compose up} in {cyan /home/user/git-local-devops/firecow-example}`,
		);
	});

	test("Start firecow.dk, failure in script", async () => {
		when(spawnSpy).calledWith(
			"docker-compose", ["up"], expect.objectContaining({}),
		).mockRejectedValue({stderr: "ARRRG FAILURE"});
		await runScripts(cwdStub, projectStub, "start", "firecow.dk");
		expect(console.error).toHaveBeenCalledWith(
			chalk`"start" "firecow.dk" failed, goto {cyan /home/user/git-local-devops/firecow-example} and run {blue docker-compose up} manually`,
		);
	});

});

describe("Git Operations", () => {

	beforeEach(() => {
		pathExistsSpy = jest.spyOn(fs, "pathExists").mockResolvedValue(true);
	});

	test("Changes found", async () => {
		await gitOperations(cwdStub, projectStub);
		expect(console.log).toHaveBeenCalledWith(
			chalk`Local changes found, no git operations will be applied in {cyan ${cwdStub}/firecow-example}`,
		);
	});

	test("Cloning project", async () => {
		pathExistsSpy = jest.spyOn(fs, "pathExists").mockResolvedValue(false);
		await gitOperations(cwdStub, projectStub);
		expect(spawnSpy).toHaveBeenCalledWith(
			"git", ["clone", "git@gitlab.com:firecow/example.git", "/home/user/git-local-devops/firecow-example"],
			expect.objectContaining({}),
		);
	});

	describe("Default branch", () => {
		test("Already up to date", async () => {
			mockHasNoChanges();
			when(spawnSpy).calledWith(
				"git", ["pull"], expect.objectContaining({}),
			).mockResolvedValue({stdout: "Already up to date."});
			await gitOperations(cwdStub, projectStub);
			expect(console.log).toHaveBeenCalledWith(chalk`Already up to date {cyan ${cwdStub}/firecow-example}`);
			expect(spawnSpy).toHaveBeenCalledWith(
				"git", ["pull"],
				expect.objectContaining({}),
			);
		});

		test("Pulling latest changes", async () => {
			mockHasNoChanges();
			await gitOperations(cwdStub, projectStub);
			expect(console.log).toHaveBeenCalledWith(chalk`Pulled {magenta origin/main} in {cyan ${cwdStub}/firecow-example}`);
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
			await gitOperations(cwdStub, projectStub);
			expect(console.log).toHaveBeenCalledWith(
				chalk`Rebased {yellow custom} on top of {magenta origin/main} in {cyan ${cwdStub}/firecow-example}`,
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
			await gitOperations(cwdStub, projectStub);
			expect(console.log).toHaveBeenCalledWith(
				chalk`Merged {magenta origin/main} with {yellow custom} in {cyan ${cwdStub}/firecow-example}`,
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
			await gitOperations(cwdStub, projectStub);
			expect(console.log).toHaveBeenCalledWith(
				chalk`Merged failed in {cyan ${cwdStub}/firecow-example}`,
			);
			expect(spawnSpy).toHaveBeenCalledWith(
				"git", ["merge", `--abort`],
				expect.objectContaining({}),
			);
		});
	});
});
