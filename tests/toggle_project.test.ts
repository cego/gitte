import fs from "fs-extra";
import { when } from "jest-when";
import { cnfStub, cwdStub } from "./utils/stubs";
import _ from "lodash";
import { projectsToggleFileName, resetToggledProjects, toggleProject } from "../src/toggle_projects";
import path from "path";

const projectsToggleMockName = path.join(cwdStub, projectsToggleFileName);

beforeEach(() => {
	fs.pathExistsSync = jest.fn();
	fs.readFileSync = jest.fn();
	fs.writeFileSync = jest.fn();
	fs.readJsonSync = jest.fn();
	console.log = jest.fn();
	console.error = jest.fn();
});

describe("resetDisabledProjects", () => {
	test("resets disabled projects", async () => {
		const config = _.cloneDeep(cnfStub);
		config.projects["projecta"].defaultDisabled = true;

		when(fs.readFileSync).calledWith(projectsToggleMockName, "utf8").mockReturnValue(`projectd`);
		when(fs.pathExistsSync).calledWith(projectsToggleMockName).mockReturnValue(true);

		resetToggledProjects(config);

		expect(fs.writeFileSync).toHaveBeenNthCalledWith(1, projectsToggleMockName, ``, "utf8");
	});
});

describe("toggleProject", () => {
	test("enable a project", () => {
		const config = _.cloneDeep(cnfStub);

		when(fs.readFileSync).calledWith(projectsToggleMockName, "utf8").mockReturnValue(`projecta:false`);
		when(fs.pathExistsSync).calledWith(projectsToggleMockName).mockReturnValue(true);

		toggleProject(config, "disabledProject");

		expect(fs.writeFileSync).toHaveBeenLastCalledWith(
			projectsToggleMockName,
			`projecta:false\ndisabledProject:true`,
			"utf8",
		);
	});

	test("disable a project", () => {
		const config = _.cloneDeep(cnfStub);

		when(fs.readFileSync).calledWith(projectsToggleMockName, "utf8").mockReturnValue(``);
		when(fs.pathExistsSync).calledWith(projectsToggleMockName).mockReturnValue(true);

		toggleProject(config, "projecta");

		expect(fs.writeFileSync).toHaveBeenLastCalledWith(projectsToggleMockName, `projecta:false`, "utf8");
	});
});
