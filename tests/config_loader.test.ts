import fs from "fs-extra";
import { when } from "jest-when";
import yaml from "js-yaml";
import * as utils from "../src/utils";
import { cnfStub, cwdStub, projectStub, startupStub } from "./utils/stubs";
import { loadConfig } from "../src/config_loader";
import { ExecaReturnValue } from "execa";
import { Config } from "../src/types/config";
import * as _ from "lodash";
import path from "path";
import { projectsToggleFileName } from "../src/toggle_projects";

const projectsToggleMockName = path.join(cwdStub, projectsToggleFileName);

let spawnSpy: ((...args: any[]) => any) | jest.MockInstance<any, any[]>;
let readSpy: jest.MockInstance<any, any[]>;
let readSpySync: jest.MockInstance<any, any[]>;
beforeEach(() => {
	readSpy = jest.spyOn(fs, "readFile");
	readSpySync = jest.spyOn(fs, "readFileSync");

	// @ts-ignore
	when(readSpy)
		.calledWith(`${cwdStub}/.gitte.yml`, "utf8")
		.mockResolvedValue(
			`---\n${yaml.dump({
				projects: { example: projectStub },
				startup: startupStub,
			})}`,
		);

	// @ts-ignore
	utils.spawn = jest.fn();
	fs.pathExists = jest.fn().mockImplementation(() => Promise.resolve(true));
	fs.writeFileSync = jest.fn().mockImplementation(() => {
		return;
	});
	fs.writeJsonSync = jest.fn().mockImplementation(() => {
		return;
	});
	console.log = jest.fn();
	console.error = jest.fn();

	spawnSpy = jest
		.spyOn(utils, "spawn")
		.mockResolvedValue({ stdout: "Mocked Stdout" } as unknown as ExecaReturnValue<string>);

	when(spawnSpy)
		.calledWith("git", ["branch", "--show-current"], expect.objectContaining({ cwd: expect.any(String) }))
		.mockResolvedValue({ stdout: "main" });

	// @ts-ignore
	when(fs.pathExists).calledWith(`${cwdStub}/.gitte-projects-disable`).mockResolvedValue(true);

	when(readSpySync).calledWith(projectsToggleMockName, "utf8").mockReturnValue(`projectd`);

	// @ts-ignore
	when(readSpySync).calledWith(`${cwdStub}/.gitte-cache.json`, "utf8").mockResolvedValue(``);

	// @ts-ignore
	when(readSpy).calledWith(`${cwdStub}/.gitte-override.yml`, "utf8").mockResolvedValue(``);
});

describe("Config loader", () => {
	test(".gitte.yml exists", async () => {
		const fileCnt = `---\n${yaml.dump({
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

	test("It merged override config", async () => {
		const fileCnt = `---\n${yaml.dump({
			startup: startupStub,
			projects: { example: projectStub },
		})}`;
		// @ts-ignore
		when(fs.pathExists).calledWith(`${cwdStub}/.gitte-env`).mockResolvedValue(false);
		// @ts-ignore
		when(fs.readFile).calledWith(`${cwdStub}/.gitte.yml`, "utf8").mockResolvedValue(fileCnt);

		const overrideFileCnt = `---\n${yaml.dump({
			startup: { ...startupStub, world: { cmd: ["echo", "world2"] } },
			projects: { example: projectStub },
		})}`;
		// @ts-ignore
		when(fs.pathExists).calledWith(`${cwdStub}/.gitte-override.yml`).mockResolvedValue(true);
		// @ts-ignore
		when(readSpy).calledWith(`${cwdStub}/.gitte-override.yml`, "utf8").mockResolvedValue(overrideFileCnt);

		const config = await loadConfig(cwdStub);

		expect(config.startup.world).toEqual({ cmd: ["echo", "world2"] });
	});

	test("It sets needs priority and searchfor if undefined", async () => {
		const fileCnt = `---\n${yaml.dump({
			startup: startupStub,
			projects: {
				example: {
					remote: "git@gitlab.com:cego/example.git",
					default_branch: "main",
					actions: {
						up: { groups: { default: ["echo", "up"] } },
					},
				},
			},
		})}`;
		// @ts-ignore
		when(fs.pathExists).calledWith(`${cwdStub}/.gitte-env`).mockResolvedValue(false);
		// @ts-ignore
		when(fs.pathExists).calledWith(`${cwdStub}/.gitte-override.yml`).mockResolvedValue(false);
		// @ts-ignore
		when(fs.readFile).calledWith(`${cwdStub}/.gitte.yml`, "utf8").mockResolvedValue(fileCnt);

		const config = await loadConfig(cwdStub);

		expect(config.projects.example.actions.up.needs).toEqual([]);
		expect(config.projects.example.actions.up.searchFor).toEqual([]);
		expect(config.projects.example.actions.up.priority).toEqual(null);
	});

	test("It removed projects from .gitte-projects-toggled", async () => {
		const fileCnt = `---\n${yaml.dump({
			startup: startupStub,
			projects: {
				example1: {
					remote: "git@gitlab.com:cego/example.git",
					default_branch: "main",
					actions: {
						up: { groups: { default: ["echo", "up"] } },
					},
				},
				example2: {
					remote: "git@gitlab.com:cego/example.git",
					default_branch: "main",
					actions: {
						up: { groups: { default: ["echo", "up"] } },
					},
				},
				example3: {
					remote: "git@gitlab.com:cego/example.git",
					default_branch: "main",
					actions: {
						up: { groups: { default: ["echo", "up"] } },
					},
				},
			},
		})}`;
		// @ts-ignore
		when(fs.pathExists).calledWith(`${cwdStub}/.gitte-env`).mockResolvedValue(false);
		// @ts-ignore
		when(fs.pathExists).calledWith(`${cwdStub}/.gitte-override.yml`).mockResolvedValue(false);
		// @ts-ignore
		when(fs.readFile).calledWith(`${cwdStub}/.gitte.yml`, "utf8").mockResolvedValue(fileCnt);
		// @ts-ignore
		when(fs.pathExists).calledWith(`${cwdStub}/.gitte-projects-disable`).mockResolvedValue(true);
		// @ts-ignore
		when(readSpySync).calledWith(projectsToggleMockName, "utf8").mockReturnValue(`example1:false\nexample3:false`);

		const config = await loadConfig(cwdStub);

		expect(Object.keys(config.projects)).toEqual(["example2"]);
	});
	test("It added projects from .gitte-projects-toggled", async () => {
		// @ts-ignore
		when(fs.pathExists).calledWith(`${cwdStub}/.gitte-env`).mockResolvedValue(false);
		// @ts-ignore
		when(fs.pathExists).calledWith(`${cwdStub}/.gitte-override.yml`).mockResolvedValue(false);

		const cnfYaml = `---\n${yaml.dump(cnfStub)}`;
		// @ts-ignore
		when(fs.readFile).calledWith(`${cwdStub}/.gitte.yml`, "utf8").mockResolvedValue(cnfYaml);
		// @ts-ignore
		when(fs.pathExists).calledWith(`${cwdStub}/.gitte-projects-disable`).mockResolvedValue(true);
		// @ts-ignore
		when(readSpySync).calledWith(projectsToggleMockName, "utf8").mockReturnValue(`disabledProject:true`);

		const config = await loadConfig(cwdStub);

		expect(Object.keys(config.projects)).toEqual(["projecta", "projectd", "projecte", "disabledProject"]);
	});
	test("It creates empty .gitte-projects-toggled if not exists", async () => {
		// @ts-ignore
		when(fs.pathExists).calledWith(`${cwdStub}/.gitte-env`).mockResolvedValue(false);
		// @ts-ignore
		when(fs.pathExists).calledWith(`${cwdStub}/.gitte-override.yml`).mockResolvedValue(false);
		// @ts-ignore
		when(fs.pathExists).calledWith(projectsToggleMockName).mockResolvedValue(false);

		const writeSpy = jest.spyOn(fs, "writeFileSync").mockImplementation(() => {
			return;
		});

		await loadConfig(cwdStub);

		expect(writeSpy).toHaveBeenCalledWith(projectsToggleMockName, "", "utf8");
	});
	test("It does not remove any projects if .gitte-projects-toggled is empty", async () => {
		const fileCnt = `---\n${yaml.dump({
			startup: startupStub,
			projects: {
				example1: {
					remote: "git@gitlab.com:cego/example.git",
					default_branch: "main",
					actions: {
						up: { groups: { default: ["echo", "up"] } },
					},
				},
				example2: {
					remote: "git@gitlab.com:cego/example.git",
					default_branch: "main",
					actions: {
						up: { groups: { default: ["echo", "up"] } },
					},
				},
				example3: {
					remote: "git@gitlab.com:cego/example.git",
					default_branch: "main",
					actions: {
						up: { groups: { default: ["echo", "up"] } },
					},
				},
			},
		})}`;
		// @ts-ignore
		when(fs.pathExists).calledWith(`${cwdStub}/.gitte-env`).mockResolvedValue(false);
		// @ts-ignore
		when(fs.pathExists).calledWith(`${cwdStub}/.gitte-override.yml`).mockResolvedValue(false);
		// @ts-ignore
		when(fs.readFile).calledWith(`${cwdStub}/.gitte.yml`, "utf8").mockResolvedValue(fileCnt);

		const config = await loadConfig(cwdStub);

		expect(Object.keys(config.projects)).toEqual(["example1", "example2", "example3"]);
	});

	test("It removes disabled projects from needs", async () => {
		// @ts-ignore
		when(fs.pathExists).calledWith(`${cwdStub}/.gitte-env`).mockResolvedValue(false);

		const cnf: Config = _.cloneDeep(cnfStub);

		cnf.projects.projecta.actions.start.needs = ["projecte"];
		// @ts-ignore
		when(fs.readFile)
			// @ts-ignore
			.calledWith(`${cwdStub}/.gitte.yml`, "utf8")
			// @ts-ignore
			.mockResolvedValue(`---\n${yaml.dump(cnf)}`);

		// @ts-ignore
		when(fs.readFileSync).calledWith(projectsToggleMockName, "utf8").mockReturnValue(`projecte:false`);

		const result = await loadConfig(cwdStub);

		expect(result.projects.projecta.actions.start.needs).toEqual([]);
	});
});
