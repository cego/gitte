// @ts-nocheck
import { getProjectDirFromRemote } from "../src/project";
import { actions } from "../src/actions";
import { gitops } from "../src/gitops";
import fs from "fs-extra";
import chalk from "chalk";
import { when } from "jest-when";
import { startup } from "../src/startup";
import * as pcp from "promisify-child-process";
import { printLogs } from "../src/utils";

function mockHasNoChanges() {
	when(pcp.spawn)
		.calledWith("git", ["status", "--porcelain"], expect.objectContaining({}))
		.mockResolvedValue({ stdout: "" });
}

function mockHasChanges() {
	when(pcp.spawn)
		.calledWith("git", ["status", "--porcelain"], expect.objectContaining({}))
		.mockResolvedValue({ stdout: " M somefile.yml\n" });
}

function mockMainBranch() {
	when(pcp.spawn)
		.calledWith("git", ["branch", "--show-current"], expect.objectContaining({ cwd: expect.any(String) }))
		.mockResolvedValue({ stdout: "main" });
}

function mockCustomBranch() {
	when(pcp.spawn)
		.calledWith("git", ["branch", "--show-current"], expect.objectContaining({ cwd: expect.any(String) }))
		.mockResolvedValue({ stdout: "custom" });
}

function mockRebaseFailed() {
	when(pcp.spawn)
		.calledWith("git", ["rebase", `origin/main`], expect.objectContaining({}))
		.mockRejectedValue("Rebase wasn't possible");
}

function mockMergeFailed() {
	when(pcp.spawn)
		.calledWith("git", ["merge", `origin/main`], expect.objectContaining({}))
		.mockRejectedValue("Merge wasn't possible");
}

function mockDockerComposeUpFail() {
	when(pcp.spawn)
		.calledWith("docker-compose", ["up"], expect.objectContaining({}))
		.mockRejectedValue({ stderr: "ARRRG FAILURE" });
}

let cwdStub, projectStub, startupStub;
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
					"example.com": ["scp", "user@example.com", "sh", "-c", "service", "webserver", "start"],
				},
			},
			down: {
				groups: {
					"cego.dk": ["docker-compose", "down"],
					"example.com": ["scp", "user@example.com", "sh", "-c", "service", "webserver", "stop"],
				},
			},
		},
	};
	startupStub = {
		world: { cmd: ["echo", "world"] },
		bashWorld: { shell: "bash", script: "echo world" },
	};

	fs.readFile = jest.fn().mockImplementation(() => Promise.resolve());
	pcp.spawn = jest.fn().mockImplementation(() => Promise.resolve({ stdout: "", stderr: "", exitCode: 0 }));
	console.log = jest.fn();
	console.error = jest.fn();
	fs.pathExists = jest.fn().mockImplementation(() => Promise.resolve(true));
});

describe("Startup checks", () => {
	test("failing argv", async () => {
		when(pcp.spawn).calledWith("echo", ["hello"], expect.objectContaining({})).mockRejectedValue(new Error("WHAT"));
		await expect(startup([{ cmd: ["echo", "hello"] }])).rejects.toThrow("WHAT");
	});

	test("failing shell", async () => {
		when(pcp.spawn)
			.calledWith("echo hello", [], expect.objectContaining({ shell: "bash" }))
			.mockRejectedValue(new Error("WHAT"));
		await expect(startup([{ shell: "bash", script: "echo hello" }])).rejects.toThrow("WHAT");
	});
});

describe("Project dir from remote", () => {
	test("Valid ssh remote", () => {
		const dir = getProjectDirFromRemote(cwdStub, "git@gitlab.com:cego/example.git");
		expect(dir).toEqual(`${cwdStub}/cego-example`);
	});

	test("Valid ssh remote with cwd ending in slash", () => {
		const dir = getProjectDirFromRemote(`${cwdStub}/`, "git@gitlab.com:cego/example.git");
		expect(dir).toEqual(`${cwdStub}/cego-example`);
	});

	test("Invalid remote", () => {
		expect(() => {
			getProjectDirFromRemote(cwdStub, "git@gitlab.coinvalidirecow/example.git");
		}).toThrowError(
			"git@gitlab.coinvalidirecow/example.git is not a valid project remote. Use git@gitlab.com:example/cego.git syntax",
		);
	});
});

describe("Run scripts", () => {
	test("Start cego.dk", async () => {
		const actOpt = {
			cwd: cwdStub,
			project: projectStub,
			currentPrio: 0,
			actionToRun: "start",
			groupToRun: "cego.dk",
		};
		await actions(actOpt);
		expect(console.log).toHaveBeenCalledWith(
			chalk`{blue docker-compose up} is running in {cyan /home/user/git-local-devops/cego-example}`,
		);
	});

	test("Start cego.dk, failure in script", async () => {
		const actOpt = {
			cwd: cwdStub,
			project: projectStub,
			currentPrio: 0,
			actionToRun: "start",
			groupToRun: "cego.dk",
		};
		mockDockerComposeUpFail();
		await actions(actOpt);
		expect(console.error).toHaveBeenCalledWith(
			chalk`"start" "cego.dk" {red failed}, goto {cyan /home/user/git-local-devops/cego-example} and run {blue docker-compose up} manually`,
		);
	});
});

describe("Git Operations", () => {
	beforeEach(() => {
		mockMainBranch();
		mockHasNoChanges();
	});

	test("Changes found", async () => {
		mockHasChanges();
		const logs = await gitops(cwdStub, projectStub);
		expect(logs).toContain(chalk`{yellow main} has local changes in {cyan ${cwdStub}/cego-example}`);
	});

	test("Cloning project", async () => {
		when(fs.pathExists).mockResolvedValue(false);
		await gitops(cwdStub, projectStub);
		expect(pcp.spawn).toHaveBeenCalledWith(
			"git",
			["clone", "git@gitlab.com:cego/example.git", "/home/user/git-local-devops/cego-example"],
			expect.objectContaining({}),
		);
	});

	describe("Default branch", () => {
		test("No remote", async () => {
			when(pcp.spawn).calledWith("git", ["pull"], expect.objectContaining({})).mockRejectedValue({
				stderr: "There is no tracking information for the current branch",
			});

			const logs = await gitops(cwdStub, projectStub);

			expect(logs).toContain(chalk`{yellow main} doesn't have a remote origin {cyan ${cwdStub}/cego-example}`);
		});

		test("Already up to date", async () => {
			when(pcp.spawn)
				.calledWith("git", ["pull"], expect.objectContaining({}))
				.mockResolvedValue({ stdout: "Already up to date." });
			const logs = await gitops(cwdStub, projectStub);
			expect(logs).toContain(chalk`{yellow main} is up to date in {cyan ${cwdStub}/cego-example}`);
			expect(pcp.spawn).toHaveBeenCalledWith("git", ["pull"], expect.objectContaining({}));
		});

		test("Pulling latest changes", async () => {
			const logs = await gitops(cwdStub, projectStub);
			expect(logs).toContain(
				chalk`{yellow main} pulled changes from {magenta origin/main} in {cyan ${cwdStub}/cego-example}`,
			);
			expect(pcp.spawn).toHaveBeenCalledWith("git", ["pull"], expect.objectContaining({}));
		});

		test("Conflicts with origin", async () => {
			when(pcp.spawn)
				.calledWith("git", ["pull"], expect.objectContaining({}))
				.mockRejectedValue({ stderr: "I'M IN CONFLICT" });

			const logs = await gitops(cwdStub, projectStub);
			expect(logs).toContain(
				chalk`{yellow main} {red conflicts} with {magenta origin/main} {cyan ${cwdStub}/cego-example}`,
			);
		});
	});

	describe("Custom branch", () => {
		test("Merging failed", async () => {
			mockHasNoChanges();
			mockCustomBranch();
			mockRebaseFailed();
			mockMergeFailed();
			const logs = await gitops(cwdStub, projectStub);
			expect(logs).toContain(
				chalk`{yellow custom} merge with {magenta origin/main} {red failed} in {cyan ${cwdStub}/cego-example}`,
			);
			expect(pcp.spawn).toHaveBeenCalledWith("git", ["merge", `--abort`], expect.objectContaining({}));
		});
	});
});

describe("Print logs", () => {
	test("It logs all successful", async () => {
		const projectNames = ["test1", "test2"];
		const logs: any[][] = [["log1", "log2"], ["log3"]];

		printLogs(projectNames, logs);

		expect(console.log).toHaveBeenCalledTimes(5);
		expect(console.log).toHaveBeenCalledWith(chalk`┌─ {green {bold test1}}`);
		expect(console.log).toHaveBeenCalledWith(`├─── log1`);
		expect(console.log).toHaveBeenCalledWith(`└─── log2`);
		expect(console.log).toHaveBeenCalledWith(chalk`┌─ {green {bold test2}}`);
		expect(console.log).toHaveBeenCalledWith(`└─── log3`);
	});

	test("It logs all failed", async () => {
		const projectNames = ["test1", "test2"];
		const logs: any[] = [new Error("test error 1"), new Error("test error 2")];

		expect(() => printLogs(projectNames, logs)).toThrowError("At least one git operation failed");

		expect(console.log).toHaveBeenCalledTimes(4);
		expect(console.log).toHaveBeenCalledWith(chalk`┌─ {red {bold test1}}`);
		expect(console.log).toHaveBeenCalledWith(expect.stringContaining("Error: test error 1"));
		expect(console.log).toHaveBeenCalledWith(chalk`┌─ {red {bold test2}}`);
		expect(console.log).toHaveBeenCalledWith(expect.stringContaining("Error: test error 2"));
	});

	test("It logs all failed and successful", async () => {
		const projectNames = ["test1", "test2"];
		const logs: any[] = [new Error("test error 1"), ["log3"]];

		expect(() => printLogs(projectNames, logs)).toThrowError("At least one git operation failed");

		expect(console.log).toHaveBeenCalledTimes(4);
		expect(console.log).toHaveBeenCalledWith(chalk`┌─ {red {bold test1}}`);
		expect(console.log).toHaveBeenCalledWith(expect.stringContaining("Error: test error 1"));
		expect(console.log).toHaveBeenCalledWith(chalk`┌─ {green {bold test2}}`);
		expect(console.log).toHaveBeenCalledWith(`└─── log3`);
	});
});
