import { when } from "jest-when";
import { gitOperations } from "../src/git_operations";
import fs from "fs-extra";
import chalk from "chalk";
import * as pcp from "promisify-child-process";
import { projectStub, cwdStub } from "./utils/stubs";

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

describe("Git Operations", () => {
	beforeEach(() => {
        // @ts-ignore
		jest.spyOn(fs, "pathExists").mockResolvedValue(true);
	});

    test("Current branch failed", async () => {
        when(spawnSpy)
            .calledWith("git", ["branch", "--show-current"], expect.objectContaining({ cwd: expect.any(String) }))
            .mockRejectedValue(new Error("WHAT"));
        
        const res = await gitOperations(cwdStub, projectStub);

        expect(res).toHaveLength(2);
        expect(res[0]).toBe(chalk`{yellow git@gitlab.com:cego/example.git} {red failed} in {cyan /home/user/git-local-devops/cego-example} Error: WHAT`);
        expect(res[1]).toBeUndefined();
    });

	test("Changes found", async () => {
		const logs = await gitOperations(cwdStub, projectStub);
		expect(logs).toContain(chalk`{yellow main} has local changes in {cyan ${cwdStub}/cego-example}`);
	});

	test("Cloning project", async () => {
        // @ts-ignore
		jest.spyOn(fs, "pathExists").mockResolvedValue(false);
		await gitOperations(cwdStub, projectStub);
		expect(spawnSpy).toHaveBeenCalledWith(
			"git",
			["clone", "git@gitlab.com:cego/example.git", "/home/user/git-local-devops/cego-example"],
			expect.objectContaining({}),
		);
	});

	describe("Default branch", () => {
		test("No remote", async () => {
			mockHasNoChanges();
			when(spawnSpy).calledWith("git", ["pull"], expect.objectContaining({})).mockRejectedValue({
				stderr: "There is no tracking information for the current branch",
			});

			const logs = await gitOperations(cwdStub, projectStub);

			expect(logs).toContain(chalk`{yellow main} doesn't have a remote origin {cyan ${cwdStub}/cego-example}`);
		});

		test("Already up to date", async () => {
			mockHasNoChanges();
			when(spawnSpy)
				.calledWith("git", ["pull"], expect.objectContaining({}))
				.mockResolvedValue({ stdout: "Already up to date." });
			const logs = await gitOperations(cwdStub, projectStub);
			expect(logs).toContain(chalk`{yellow main} is up to date in {cyan ${cwdStub}/cego-example}`);
			expect(spawnSpy).toHaveBeenCalledWith("git", ["pull"], expect.objectContaining({}));
		});

		test("Pulling latest changes", async () => {
			mockHasNoChanges();
			const logs = await gitOperations(cwdStub, projectStub);
			expect(logs).toContain(
				chalk`{yellow main} pulled changes from {magenta origin/main} in {cyan ${cwdStub}/cego-example}`,
			);
			expect(spawnSpy).toHaveBeenCalledWith("git", ["pull"], expect.objectContaining({}));
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
			expect(spawnSpy).toHaveBeenCalledWith("git", ["rebase", `origin/main`], expect.objectContaining({}));
		});

		test("Rebasing, already up to date", async () => {
			mockHasNoChanges();
			mockCustomBranch();
			when(spawnSpy)
				.calledWith("git", ["rebase", "origin/main"], expect.objectContaining({}))
				.mockResolvedValue({ stdout: "Current branch custom is up to date." });

			const logs = await gitOperations(cwdStub, projectStub);
			expect(logs).toContain(
				chalk`{yellow custom} is already on {magenta origin/main} in {cyan ${cwdStub}/cego-example}`,
			);
			expect(spawnSpy).toHaveBeenCalledWith("git", ["rebase", `origin/main`], expect.objectContaining({}));
		});

		test("Rebase failed. Merging", async () => {
			mockHasNoChanges();
			mockCustomBranch();
			mockRebaseFailed();
			const logs = await gitOperations(cwdStub, projectStub);
			expect(logs).toContain(
				chalk`{yellow custom} was merged with {magenta origin/main} in {cyan ${cwdStub}/cego-example}`,
			);
			expect(spawnSpy).toHaveBeenCalledWith("git", ["rebase", `--abort`], expect.objectContaining({}));
			expect(spawnSpy).toHaveBeenCalledWith("git", ["merge", `origin/main`], expect.objectContaining({}));
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
			expect(spawnSpy).toHaveBeenCalledWith("git", ["merge", `--abort`], expect.objectContaining({}));
		});
	});
});
