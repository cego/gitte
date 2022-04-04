import { when } from "jest-when";
import { gitops } from "../src/gitops";
import fs from "fs-extra";
import chalk from "chalk";
import * as pcp from "promisify-child-process";
import { projectStub, cwdStub } from "./utils/stubs";
import { ErrorWithHint } from "../src/types/utils";

let spawnSpy: ((...args: any[]) => any) | jest.MockInstance<any, any[]>;
beforeEach(() => {
	// @ts-ignore
	pcp.spawn = jest.fn();
	console.log = jest.fn();
	console.error = jest.fn();
	fs.pathExists = jest.fn();

	spawnSpy = jest.spyOn(pcp, "spawn").mockResolvedValue({ stdout: "Mocked Stdout" });

	when(spawnSpy)
		.calledWith("git", ["branch", "--show-current"], expect.objectContaining({ cwd: expect.any(String) }))
		.mockResolvedValue({ stdout: "main" });
});

function mockHasNoChanges() {
	when(spawnSpy)
		.calledWith("git", ["status", "--porcelain"], expect.objectContaining({}))
		.mockResolvedValue({ stdout: "" });
}

function mockCustomBranch() {
	when(spawnSpy)
		.calledWith("git", ["branch", "--show-current"], expect.objectContaining({ cwd: expect.any(String) }))
		.mockResolvedValue({ stdout: "custom" });
}

function mockMergeFailed() {
	when(spawnSpy)
		.calledWith("git", ["merge", `origin/main`], expect.objectContaining({}))
		.mockRejectedValue("Merge wasn't possible");
}

function mockMergeAbortFailed() {
	when(spawnSpy)
		.calledWith("git", ["merge", "--abort"], expect.objectContaining({}))
		.mockRejectedValue("Merge --abort wasn't possible");
}

describe("Git Operations", () => {
	beforeEach(() => {
		// @ts-ignore
		jest.spyOn(fs, "pathExists").mockResolvedValue(true);
	});

	test("Current branch failed", async () => {
		when(spawnSpy)
			.calledWith("git", ["branch", "--show-current"], expect.objectContaining({ cwd: expect.any(String) }))
			.mockRejectedValue(new Error("WHAT"));

		const res = await gitops(cwdStub, projectStub);

		expect(res).toHaveLength(1);
		const msg = chalk`{yellow git@gitlab.com:cego/example.git} {red failed} in {cyan /home/user/git-local-devops/cego-example} Error: WHAT`;
		expect(res[0]).toBeInstanceOf(ErrorWithHint);
		expect((res[0] as ErrorWithHint).message).toBe(msg);
	});

	test("Changes found", async () => {
		const logs = await gitops(cwdStub, projectStub);
		expect(logs).toContain(chalk`{yellow main} has local changes in {cyan ${cwdStub}/cego-example}`);
	});

	test("Cloning project", async () => {
		// @ts-ignore
		jest.spyOn(fs, "pathExists").mockResolvedValue(false);
		await gitops(cwdStub, projectStub);
		expect(spawnSpy).toHaveBeenCalledWith(
			"git",
			["clone", "git@gitlab.com:cego/example.git", "/home/user/git-local-devops/cego-example"],
			expect.objectContaining({}),
		);
	});

	describe("Default branch", () => {
		test("No remote", async () => {
			mockHasNoChanges();
			when(spawnSpy).calledWith("git", ["pull", "--ff-only"], expect.objectContaining({})).mockRejectedValue({
				stderr: "There is no tracking information for the current branch",
			});

			const logs = await gitops(cwdStub, projectStub);

			expect(logs).toContain(chalk`{cyan main} {red doesn't have a remote origin} {cyan ${cwdStub}/cego-example}`);
		});

		test("Already up to date", async () => {
			mockHasNoChanges();
			when(spawnSpy)
				.calledWith("git", ["pull", "--ff-only"], expect.objectContaining({}))
				.mockResolvedValue({ stdout: "Already up to date." });
			const logs = await gitops(cwdStub, projectStub);
			const msg = chalk`{cyan main} is up to date with {magenta origin/main} in {cyan ${cwdStub}/cego-example}`;
			expect(logs).toContain(msg);
			expect(spawnSpy).toHaveBeenCalledWith("git", ["pull", "--ff-only"], expect.objectContaining({}));
		});

		test("Pulling latest changes", async () => {
			mockHasNoChanges();
			const logs = await gitops(cwdStub, projectStub);
			const msg = chalk`{cyan main} pulled changes from {magenta origin/main} in {cyan ${cwdStub}/cego-example}`;
			expect(logs).toContain(msg);
			expect(spawnSpy).toHaveBeenCalledWith("git", ["pull", "--ff-only"], expect.objectContaining({}));
		});

		test("Conflicts with origin", async () => {
			mockHasNoChanges();
			when(spawnSpy)
				.calledWith("git", ["pull", "--ff-only"], expect.objectContaining({}))
				.mockRejectedValue({ stderr: "I'M IN CONFLICT" });

			const logs = await gitops(cwdStub, projectStub);
			const msg = chalk`{cyan main} {red conflicts} with {magenta origin/main} {cyan ${cwdStub}/cego-example}`;
			expect(logs).toContain(msg);
		});
	});

	describe("Custom branch", () => {
		test("Merged successfully", async () => {
			mockHasNoChanges();
			mockCustomBranch();
			const logs = await gitops(cwdStub, projectStub);
			const msg = chalk`{yellow {cyan custom} was merged with {magenta origin/main} in {cyan ${cwdStub}/cego-example}}`;
			expect(logs).toContain(msg);
		});

		test("Already merged", async () => {
			mockHasNoChanges();
			mockCustomBranch();
			when(spawnSpy)
				.calledWith("git", ["merge", `origin/main`], expect.objectContaining({}))
				.mockResolvedValue({ stdout: "Already up to date." });
			const logs = await gitops(cwdStub, projectStub);
			const msg = chalk`{cyan custom} is up to date with {magenta origin/main} in {cyan ${cwdStub}/cego-example}`;
			expect(logs).toContain(msg);
		});

		test("Merge failed", async () => {
			mockHasNoChanges();
			mockCustomBranch();
			mockMergeFailed();
			const logs = await gitops(cwdStub, projectStub);
			const msg = chalk`{yellow custom} merge with {magenta origin/main} {red failed} in {cyan ${cwdStub}/cego-example}`;
			expect(logs).toContain(msg);
		});

		test("Merge failed, abort failed", async () => {
			mockHasNoChanges();
			mockCustomBranch();
			mockMergeFailed();
			mockMergeAbortFailed();
			const logs = await gitops(cwdStub, projectStub);
			const msg = chalk`{yellow custom} merge --abort also {red failed} in {cyan ${cwdStub}/cego-example}`;
			expect(logs).toContain(msg);
		});
	});

	describe("Clone repository", () => {
		test("Cloning project", async () => {
			// @ts-ignore
			jest.spyOn(fs, "pathExists").mockResolvedValue(false);
			const logs = await gitops(cwdStub, projectStub);
			expect(spawnSpy).toBeCalledWith(
				"git",
				["clone", "git@gitlab.com:cego/example.git", "/home/user/git-local-devops/cego-example"],
				{ cwd: cwdStub, encoding: "utf8" },
			);
			expect(logs).toHaveLength(1);
			expect(logs).toContain(
				chalk`{gray git@gitlab.com:cego/example.git} was cloned to {cyan /home/user/git-local-devops/cego-example}`,
			);
		});

		test("Cloning project failed", async () => {
			// @ts-ignore
			jest.spyOn(fs, "pathExists").mockResolvedValue(false);
			when(spawnSpy)
				.calledWith("git", ["clone", "git@gitlab.com:cego/example.git", "/home/user/git-local-devops/cego-example"], {
					cwd: cwdStub,
					encoding: "utf8",
				})
				.mockRejectedValue({ stderr: "Permission denied" });
			const logs = await gitops(cwdStub, projectStub);
			expect(logs).toHaveLength(1);
			expect(logs[0]).toBeInstanceOf(ErrorWithHint);

			expect((logs[0] as ErrorWithHint).message).toBe(
				chalk`Permission denied to clone git@gitlab.com:cego/example.git`,
			);
		});
	});
});
