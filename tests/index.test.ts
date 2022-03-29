// @ts-nocheck
import { getProjectDirFromRemote } from "../src/project";
import { runAction } from "../src/actions";
import { gitOperations } from "../src/git_operations";
import fs from "fs-extra";
import chalk from "chalk";
import { when } from "jest-when";
import { start } from "../src";
import yaml from "js-yaml";
import { startup } from "../src/startup";
import * as pcp from "promisify-child-process";

function mockHasNoChanges() {
	when(spawnSpy)
		.calledWith("git", ["status", "--porcelain"], expect.objectContaining({}))
		.mockResolvedValue({ stdout: "" });
}

function mockCustomBranch() {
	when(spawnSpy)
		.calledWith(
			"git",
			["branch", "--show-current"],
			expect.objectContaining({ cwd: expect.any(String) }),
		)
		.mockResolvedValue({ stdout: "custom" });
}

function mockRebaseFailed() {
	when(spawnSpy)
		.calledWith("git", ["rebase", `origin/main`], expect.objectContaining({}))
		.mockRejectedValue("Rebase wasn't possible");
}

function mockMergeFailed() {
	when(spawnSpy)
		.calledWith("git", ["merge", `origin/main`], expect.objectContaining({}))
		.mockRejectedValue("Merge wasn't possible");
}

let cwdStub, cnfStub, projectStub, startupStub, readFileSpy, spawnSpy;
beforeEach(() => {
	cwdStub = "/home/user/git-local-devops";
	projectStub = {
		default_branch: "main",
		remote: "git@gitlab.com:cego/example.git",
		priority: 0,
		actions: {
			start: {
				groups: {
					"cego.dk": ["docker-compose", "up"],
					"example.com": [
						"scp",
						"user@example.com",
						"sh",
						"-c",
						"service",
						"webserver",
						"start",
					],
				},
			},
			down: {
				groups: {
					"cego.dk": ["docker-compose", "down"],
					"example.com": [
						"scp",
						"user@example.com",
						"sh",
						"-c",
						"service",
						"webserver",
						"stop",
					],
				},
			},
		},
	};
	startupStub = {
		world: { cmd: ["echo", "world"] },
		bashWorld: { shell: "bash", script: "echo world" },
	};
	cnfStub = {
		startup: startupStub,
		projects: {
			projecta: projectStub,
		},
	};
	readFileSpy = jest.spyOn(fs, "readFile").mockImplementation(() => {
		return `---\n${yaml.dump({
			projects: { example: projectStub },
			startup: startupStub,
		})}`;
	});
	pcp.spawn = jest.fn();
	console.log = jest.fn();
	console.error = jest.fn();
	fs.pathExists = jest.fn();

	spawnSpy = jest.spyOn(pcp, "spawn").mockImplementation(() => {
		return Promise.resolve({ stdout: "Mocked Stdout" });
	});

	when(spawnSpy)
		.calledWith(
			"git",
			["branch", "--show-current"],
			expect.objectContaining({ cwd: expect.any(String) }),
		)
		.mockResolvedValue({ stdout: "main" });
});

describe("Index (start)", () => {
	test("with default stubs", async () => {
		when(fs.pathExists)
			.calledWith(`${cwdStub}/.git-local-devops-env`)
			.mockResolvedValue(false);
		when(fs.pathExists)
			.calledWith(`${cwdStub}/.git-local-devops.yml`)
			.mockResolvedValue(true);
		await expect(start(cwdStub)).resolves.toBe();
	});

	test(".git-local-devops-env present", async () => {
		const remoteGitFile = ".git-local-devops.yml";
		const remoteGitRepo = "git@gitlab.com:cego/example.git";
		const remoteGitRef = "main";

		when(fs.pathExists)
			.calledWith(`${cwdStub}/.git-local-devops-env`)
			.mockResolvedValue(true);
		when(fs.pathExists)
			.calledWith(`${cwdStub}/.git-local-devops.yml`)
			.mockResolvedValue(true);
		when(readFileSpy)
			.calledWith(`${cwdStub}/.git-local-devops-env`)
			.mockImplementation(() => {
				return `REMOTE_GIT_FILE="${remoteGitFile}"\nREMOTE_GIT_REPO="${remoteGitRepo}"\nREMOTE_GIT_REF="${remoteGitRef}"`;
			});
		when(spawnSpy)
			.calledWith(
				"git",
				[
					"archive",
					`--remote=${remoteGitRepo}`,
					remoteGitRef,
					remoteGitFile,
					"|",
					"tar",
					"-xO",
					remoteGitFile,
				],
				expect.objectContaining({}),
			)
			.mockResolvedValue({
				stdout: `---\n${yaml.dump({
					projects: { example: projectStub },
					startup: startupStub,
				})}`,
			});
		await expect(start(cwdStub)).resolves.toBe();
	});

	test("config file not found", async () => {
		when(fs.pathExists)
			.calledWith(`${cwdStub}/.git-local-devops.yml`)
			.mockResolvedValue(false);
		await expect(start("/home/user/completelyinvalidpath")).rejects.toThrow(
			"No .git-local-devops.yml or .git-local-devops-env found in current or parent directories.",
		);
	});
});

describe("Startup checks", () => {
	test("failing argv", async () => {
		when(spawnSpy)
			.calledWith("echo", ["hello"], expect.objectContaining({}))
			.mockRejectedValue(new Error("WHAT"));
		await expect(startup([{ cmd: ["echo", "hello"] }])).rejects.toThrow("WHAT");
	});

	test("failing shell", async () => {
		when(spawnSpy)
			.calledWith("echo hello", [], expect.objectContaining({ shell: "bash" }))
			.mockRejectedValue(new Error("WHAT"));
		await expect(
			startup([{ shell: "bash", script: "echo hello" }]),
		).rejects.toThrow("WHAT");
	});
});

describe("Project dir from remote", () => {
	test("Valid ssh remote", () => {
		const dir = getProjectDirFromRemote(
			cwdStub,
			"git@gitlab.com:cego/example.git",
		);
		expect(dir).toEqual(`${cwdStub}/cego-example`);
	});

	test("Valid ssh remote with cwd ending in slash", () => {
		const dir = getProjectDirFromRemote(
			`${cwdStub}/`,
			"git@gitlab.com:cego/example.git",
		);
		expect(dir).toEqual(`${cwdStub}/cego-example`);
	});

	test("Invalid remote", () => {
		expect(() => {
			getProjectDirFromRemote(
				cwdStub,
				"git@gitlab.coinvalidirecow/example.git",
			);
		}).toThrowError(
			"git@gitlab.coinvalidirecow/example.git is not a valid project remote. Use git@gitlab.com:example/cego.git syntax",
		);
	});
});

describe("Run scripts", () => {
	test("Start cego.dk", async () => {
		await runAction(
			cwdStub,
			cnfStub,
			{ project: "projecta", action: "start", group: "cego.dk" },
			0,
		);
		expect(console.log).toHaveBeenCalledWith(
			chalk`{blue docker-compose up} is running in {cyan /home/user/git-local-devops/cego-example}`,
		);
	});

	test("Start cego.dk, failure in script", async () => {
		when(spawnSpy)
			.calledWith("docker-compose", ["up"], expect.objectContaining({}))
			.mockRejectedValue({ stderr: "ARRRG FAILURE" });
		await runAction(
			cwdStub,
			cnfStub,
			{ project: "projecta", action: "start", group: "cego.dk" },
			0,
		);
		expect(console.error).toHaveBeenCalledWith(
			chalk`"start" "cego.dk" {red failed}, goto {cyan /home/user/git-local-devops/cego-example} and run {blue docker-compose up} manually`,
		);
	});
});

describe("Git Operations", () => {
	beforeEach(() => {
		jest.spyOn(fs, "pathExists").mockResolvedValue(true);
	});

	test("Changes found", async () => {
		const logs = await gitOperations(cwdStub, projectStub);
		expect(logs).toContain(
			chalk`{yellow main} has local changes in {cyan ${cwdStub}/cego-example}`,
		);
	});

	test("Cloning project", async () => {
		jest.spyOn(fs, "pathExists").mockResolvedValue(false);
		await gitOperations(cwdStub, projectStub);
		expect(spawnSpy).toHaveBeenCalledWith(
			"git",
			[
				"clone",
				"git@gitlab.com:cego/example.git",
				"/home/user/git-local-devops/cego-example",
			],
			expect.objectContaining({}),
		);
	});

	describe("Default branch", () => {
		test("No remote", async () => {
			mockHasNoChanges();
			when(spawnSpy)
				.calledWith("git", ["pull"], expect.objectContaining({}))
				.mockRejectedValue({
					stderr: "There is no tracking information for the current branch",
				});

			const logs = await gitOperations(cwdStub, projectStub);

			expect(logs).toContain(
				chalk`{yellow main} doesn't have a remote origin {cyan ${cwdStub}/cego-example}`,
			);
		});

		test("Already up to date", async () => {
			mockHasNoChanges();
			when(spawnSpy)
				.calledWith("git", ["pull"], expect.objectContaining({}))
				.mockResolvedValue({ stdout: "Already up to date." });
			const logs = await gitOperations(cwdStub, projectStub);
			expect(logs).toContain(
				chalk`{yellow main} is up to date in {cyan ${cwdStub}/cego-example}`,
			);
			expect(spawnSpy).toHaveBeenCalledWith(
				"git",
				["pull"],
				expect.objectContaining({}),
			);
		});

		test("Pulling latest changes", async () => {
			mockHasNoChanges();
			const logs = await gitOperations(cwdStub, projectStub);
			expect(logs).toContain(
				chalk`{yellow main} pulled changes from {magenta origin/main} in {cyan ${cwdStub}/cego-example}`,
			);
			expect(spawnSpy).toHaveBeenCalledWith(
				"git",
				["pull"],
				expect.objectContaining({}),
			);
		});

		test("Conflicts with origin", async () => {
			mockHasNoChanges();
			when(spawnSpy)
				.calledWith("git", ["pull"], expect.objectContaining({}))
				.mockRejectedValue({ stderr: "I'M IN CONFLICT" });

			const logs = await gitOperations(cwdStub, projectStub);
			expect(logs).toContain(
				chalk`{yellow main} {red conflicts} with {magenta origin/main} {cyan ${cwdStub}/cego-example}`,
			);
		});
	});

	describe("Custom branch", () => {
		test("Rebasing", async () => {
			mockHasNoChanges();
			mockCustomBranch();
			const logs = await gitOperations(cwdStub, projectStub);
			expect(logs).toContain(
				chalk`{yellow custom} was rebased on {magenta origin/main} in {cyan ${cwdStub}/cego-example}`,
			);
			expect(spawnSpy).toHaveBeenCalledWith(
				"git",
				["rebase", `origin/main`],
				expect.objectContaining({}),
			);
		});

		test("Rebasing, already up to date", async () => {
			mockHasNoChanges();
			mockCustomBranch();
			when(spawnSpy)
				.calledWith(
					"git",
					["rebase", "origin/main"],
					expect.objectContaining({}),
				)
				.mockResolvedValue({ stdout: "Current branch custom is up to date." });

			const logs = await gitOperations(cwdStub, projectStub);
			expect(logs).toContain(
				chalk`{yellow custom} is already on {magenta origin/main} in {cyan ${cwdStub}/cego-example}`,
			);
			expect(spawnSpy).toHaveBeenCalledWith(
				"git",
				["rebase", `origin/main`],
				expect.objectContaining({}),
			);
		});

		test("Rebase failed. Merging", async () => {
			mockHasNoChanges();
			mockCustomBranch();
			mockRebaseFailed();
			const logs = await gitOperations(cwdStub, projectStub);
			expect(logs).toContain(
				chalk`{yellow custom} was merged with {magenta origin/main} in {cyan ${cwdStub}/cego-example}`,
			);
			expect(spawnSpy).toHaveBeenCalledWith(
				"git",
				["rebase", `--abort`],
				expect.objectContaining({}),
			);
			expect(spawnSpy).toHaveBeenCalledWith(
				"git",
				["merge", `origin/main`],
				expect.objectContaining({}),
			);
		});

		test("Merging failed", async () => {
			mockHasNoChanges();
			mockCustomBranch();
			mockRebaseFailed();
			mockMergeFailed();
			const logs = await gitOperations(cwdStub, projectStub);
			expect(logs).toContain(
				chalk`{yellow custom} merge with {magenta origin/main} {red failed} in {cyan ${cwdStub}/cego-example}`,
			);
			expect(spawnSpy).toHaveBeenCalledWith(
				"git",
				["merge", `--abort`],
				expect.objectContaining({}),
			);
		});
	});
});
