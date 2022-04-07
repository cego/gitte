import fs from "fs-extra";
import { when } from "jest-when";
import yaml from "js-yaml";
import * as utils from "../src/utils";
import { cwdStub, projectStub, startupStub } from "./utils/stubs";
import { loadConfig } from "../src/config_loader";
import { ExecaReturnValue } from "execa";

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
	utils.spawn = jest.fn();
	fs.pathExists = jest.fn().mockImplementation(() => Promise.resolve(true));
	console.log = jest.fn();
	console.error = jest.fn();

	spawnSpy = jest
		.spyOn(utils, "spawn")
		.mockResolvedValue({ stdout: "Mocked Stdout" } as unknown as ExecaReturnValue<string>);

	when(spawnSpy)
		.calledWith("git", ["branch", "--show-current"], expect.objectContaining({ cwd: expect.any(String) }))
		.mockResolvedValue({ stdout: "main" });
});

describe("Config loader", () => {
	test(".gitte.yml exists", async () => {
		const fileCnt = `---\n${JSON.stringify({
			startup: startupStub,
			projects: { example: projectStub },
		})}`;
		// @ts-ignore
		when(fs.pathExists).calledWith(`${cwdStub}/.gitte-env`).mockResolvedValue(false);
		// @ts-ignore
		when(fs.readFile).calledWith(`${cwdStub}/.gitte.yml`, "utf8").mockResolvedValue(fileCnt);
		await expect(loadConfig(cwdStub));
	});

	test(".gitte-env exists", async () => {
		const envFileCnt = `REMOTE_GIT_FILE="gitte.yml"\nREMOTE_GIT_REPO="git@gitlab.cego.dk:cego/local-helper-configs.git"\nREMOTE_GIT_REF="master"\n`;
		// @ts-ignore
		when(fs.pathExists).calledWith(`${cwdStub}/.gitte-env`).mockResolvedValue(true);
		// @ts-ignore
		when(fs.readFile).calledWith(`${cwdStub}/.gitte-env`, "utf8").mockResolvedValue(envFileCnt);

		const gitArchiveCnt = `---\n${yaml.dump({
			startup: startupStub,
			projects: { example: projectStub },
		})}`;
		when(utils.spawn)
			// @ts-ignore
			.calledWith("git", expect.arrayContaining(["archive"]), expect.objectContaining({}))
			.mockResolvedValue({ stdout: gitArchiveCnt } as unknown as ExecaReturnValue<string>);

		await expect(loadConfig(cwdStub));
	});
});
