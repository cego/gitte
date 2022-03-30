import fs from "fs-extra";
import { when } from "jest-when";
import { start } from "../src";
import yaml from "js-yaml";
import * as pcp from "promisify-child-process";
import { projectStub, startupStub, cwdStub } from "./utils/stubs";

let readFileSpy: ((...args: any[]) => any) | jest.MockInstance<any, any[]>;
let spawnSpy: ((...args: any[]) => any) | jest.MockInstance<any, any[]>;
beforeEach(() => {
	readFileSpy = jest.spyOn(fs, "readFile").mockResolvedValue(
		Buffer.from(
			`---\n${yaml.dump({
				projects: { example: projectStub },
				startup: startupStub,
			})}`,
		),
	);

	// @ts-ignore
	pcp.spawn = jest.fn();
	fs.pathExists = jest.fn();
	console.log = jest.fn();

	spawnSpy = jest.spyOn(pcp, "spawn").mockResolvedValue({ stdout: "Mocked Stdout" });

	when(spawnSpy)
		.calledWith("git", ["branch", "--show-current"], expect.objectContaining({ cwd: expect.any(String) }))
		.mockResolvedValue({ stdout: "main" });
});

describe("Index (start)", () => {
	test("with default stubs", async () => {
		// @ts-ignore
		when(fs.pathExists).calledWith(`${cwdStub}/.git-local-devops-env`).mockResolvedValue(false);

		// @ts-ignore
		when(fs.pathExists).calledWith(`${cwdStub}/.git-local-devops.yml`).mockResolvedValue(true);
		await expect(start(cwdStub, "", "")).resolves.toBe(undefined);
	});

	test(".git-local-devops-env present", async () => {
		const remoteGitFile = ".git-local-devops.yml";
		const remoteGitRepo = "git@gitlab.com:cego/example.git";
		const remoteGitRef = "main";

		// @ts-ignore
		when(fs.pathExists).calledWith(`${cwdStub}/.git-local-devops-env`).mockResolvedValue(true);
		// @ts-ignore
		when(fs.pathExists).calledWith(`${cwdStub}/.git-local-devops.yml`).mockResolvedValue(true);
		when(readFileSpy)
			.calledWith(`${cwdStub}/.git-local-devops-env`)
			.mockImplementation(() => {
				return `REMOTE_GIT_FILE="${remoteGitFile}"\nREMOTE_GIT_REPO="${remoteGitRepo}"\nREMOTE_GIT_REF="${remoteGitRef}"`;
			});
		when(spawnSpy)
			.calledWith(
				"git",
				["archive", `--remote=${remoteGitRepo}`, remoteGitRef, remoteGitFile, "|", "tar", "-xO", remoteGitFile],
				expect.objectContaining({}),
			)
			.mockResolvedValue({
				stdout: `---\n${yaml.dump({
					projects: { example: projectStub },
					startup: startupStub,
				})}`,
			});
		await expect(start(cwdStub, "", "")).resolves.toBe(undefined);
	});

	test("config file not found", async () => {
		// @ts-ignore
		when(fs.pathExists).calledWith(`${cwdStub}/.git-local-devops.yml`).mockResolvedValue(false);
		await expect(start("/home/user/completelyinvalidpath", "", "")).rejects.toThrow(
			"No .git-local-devops.yml or .git-local-devops-env found in current or parent directories.",
		);
	});
});
