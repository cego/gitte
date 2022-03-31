import fs from "fs-extra";
import { when } from "jest-when";
import yaml from "js-yaml";
import * as pcp from "promisify-child-process";
import { projectStub, startupStub, cwdStub } from "./utils/stubs";
import { loadConfig } from "../src/config_loader";

let spawnSpy: ((...args: any[]) => any) | jest.MockInstance<any, any[]>;
beforeEach(() => {
	jest.spyOn(fs, "readFile").mockResolvedValue(
		Buffer.from(
			`---\n${yaml.dump({
				projects: { example: projectStub },
				startup: startupStub,
			})}`,
		),
	);

	// @ts-ignore
	pcp.spawn = jest.fn();
	fs.pathExists = jest.fn().mockImplementation(() => Promise.resolve(true));
	console.log = jest.fn();
	console.error = jest.fn();

	spawnSpy = jest.spyOn(pcp, "spawn").mockResolvedValue({ stdout: "Mocked Stdout" });

	when(spawnSpy)
		.calledWith("git", ["branch", "--show-current"], expect.objectContaining({ cwd: expect.any(String) }))
		.mockResolvedValue({ stdout: "main" });
});

describe("Config loader", () => {
	test(".git-local-devops.yml exists", async () => {
		const fileCnt = `---\n${JSON.stringify({
			startup: startupStub,
			projects: { example: projectStub },
		})}`;
		// @ts-ignore
		when(fs.pathExists).calledWith(`${cwdStub}/.git-local-devops-env`).mockResolvedValue(false);
		// @ts-ignore
		when(fs.readFile).calledWith(`${cwdStub}/.git-local-devops.yml`, "utf8").mockResolvedValue(fileCnt);
		await expect(loadConfig(cwdStub));
	});

	test(".git-local-devops-env exists", async () => {
		const envFileCnt = `REMOTE_GIT_FILE="git-local-devops.yml"\nREMOTE_GIT_REPO="git@gitlab.cego.dk:cego/local-helper-configs.git"\nREMOTE_GIT_REF="master"\n`;
		// @ts-ignore
		when(fs.pathExists).calledWith(`${cwdStub}/.git-local-devops-env`).mockResolvedValue(true);
		// @ts-ignore
		when(fs.readFile).calledWith(`${cwdStub}/.git-local-devops-env`, "utf8").mockResolvedValue(envFileCnt);

		const gitArchiveCnt = `---\n${JSON.stringify({
			startup: startupStub,
			projects: { example: projectStub },
		})}`;
		when(pcp.spawn)
			// @ts-ignore
			.calledWith("git", expect.arrayContaining(["archive"]), expect.objectContaining({}))
			.mockResolvedValue({ stdout: gitArchiveCnt });

		await expect(loadConfig(cwdStub));
	});
});
