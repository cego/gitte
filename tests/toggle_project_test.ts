import fs from "fs-extra";
import { when } from "jest-when";
import { cnfStub, cwdStub } from "./utils/stubs";
import _ from "lodash";
import {
	getToggledProjects,
    logProjectStatus,
    resetToggledProjects,
    toggleProject,
} from "../src/toggle_projects";

beforeEach(() => {
	fs.pathExistsSync = jest.fn();
	fs.readFileSync = jest.fn();
	fs.writeFileSync = jest.fn();
	fs.readJsonSync = jest.fn();
	console.log = jest.fn();
	console.error = jest.fn();
});

describe("getPreviouslySeenProjectsFromCache", () => {
	test("finds projects from cache", () => {
		const config = _.cloneDeep(cnfStub);
		config.projects["projecta"].defaultDisabled = true;

		when(fs.pathExistsSync).calledWith(`${cwdStub}/.gitte-cache.json`).mockReturnValue(true);
		when(fs.readJsonSync)
			.calledWith(`${cwdStub}/.gitte-cache.json`)
			.mockReturnValue({
				version: 1,
				seenProjects: ["projecta"],
				config: cnfStub,
			});

		const projectNames = getPreviouslySeenProjectsFromCache(`${cwdStub}/.gitte-cache.json`);

		expect(projectNames).toEqual(["projecta"]);
	});
});

describe("resetDisabledProjects", () => {
	test("resets disabled projects", async () => {
		const config = _.cloneDeep(cnfStub);
		config.projects["projecta"].defaultDisabled = true;

		when(fs.readFileSync).calledWith(`${cwdStub}/.gitte-projects-disable`, "utf8").mockReturnValue(`projectd`);
		when(fs.pathExistsSync).calledWith(`${cwdStub}/.gitte-projects-disable`).mockReturnValue(true);

		resetDisabledProjects(config);

		expect(fs.writeFileSync).toHaveBeenNthCalledWith(1, `${cwdStub}/.gitte-projects-disable`, `projecta`, "utf8");
	});
});

describe("toggleProject", () => {
	test("enable a project", () => {
		const config = _.cloneDeep(cnfStub);

		when(fs.readFileSync).calledWith(`${cwdStub}/.gitte-projects-disable`, "utf8").mockReturnValue(`projecta`);
		when(fs.pathExistsSync).calledWith(`${cwdStub}/.gitte-projects-disable`).mockReturnValue(true);

		toggleProject(config, "projecta");

		expect(fs.writeFileSync).toHaveBeenLastCalledWith(`${cwdStub}/.gitte-projects-disable`, ``, "utf8");
	});

	test("disable a project", () => {
		const config = _.cloneDeep(cnfStub);

		when(fs.readFileSync).calledWith(`${cwdStub}/.gitte-projects-disable`, "utf8").mockReturnValue(``);
		when(fs.pathExistsSync).calledWith(`${cwdStub}/.gitte-projects-disable`).mockReturnValue(true);

		toggleProject(config, "projecta");

		expect(fs.writeFileSync).toHaveBeenLastCalledWith(`${cwdStub}/.gitte-projects-disable`, `projecta`, "utf8");
	});
});
