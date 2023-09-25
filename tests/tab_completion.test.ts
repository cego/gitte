import _ from "lodash";
import {
	getActionNames,
	getGroupNames,
	getProjectNames,
	getUppableGroupNames,
	tabCompleteActions,
	tabCompleteClean,
	tabCompleteSwitch,
	tabCompleteToggle,
} from "../src/tab_completion";
import { Config } from "../src/types/config";
import { cnfStub, cwdStub } from "./utils/stubs";
import fs from "fs-extra";
import { when } from "jest-when";
import path from "path";
import { projectsToggleFileName } from "../src/toggle_projects";

let config: Config = _.cloneDeep(cnfStub);
beforeEach(() => {
	config = _.cloneDeep(cnfStub);

	fs.readJsonSync = jest.fn();
	fs.pathExistsSync = jest.fn();
	fs.readFileSync = jest.fn();

	when(fs.readJsonSync).calledWith(path.join(cwdStub, ".gitte-cache.json")).mockReturnValue({
		config,
		version: 1,
		seenProjects: [],
	});

	when(fs.readFileSync).calledWith(path.join(cwdStub, projectsToggleFileName), "utf8").mockReturnValue(``);

	when(fs.pathExistsSync).calledWith(path.join(cwdStub, ".gitte-cache.json")).mockReturnValue(true);
});

describe("Action tab completion", () => {
	test("getActionNames", () => {
		expect(getActionNames(config)).toEqual(["start", "down", "up"]);
	});
	test("getGroupNames", () => {
		expect(getGroupNames(config, "start")).toEqual(["cego.dk", "example.com"]);
	});
	test("getProjectNames", () => {
		expect(getProjectNames(config, "start", "cego.dk")).toEqual(["projecta"]);
	});
	describe("tabCompleteActions", () => {
		it("should handle actions", () => {
			const res = tabCompleteActions("", { _: ["tab_completion", "start", ""], cwd: cwdStub });

			expect(res).toEqual(["all", "start", "down", "up"]);
		});

		it("should handle +", () => {
			const res = tabCompleteActions("", { _: ["tab_completion", "start", "start+down+"], cwd: cwdStub });

			expect(res).toEqual(["start+down+all", "start+down+up"]);
		});

		it("should handle groups", () => {
			const res = tabCompleteActions("", { _: ["tab_completion", "start", "up", ""], cwd: cwdStub });

			expect(res).toEqual(["all", "cego.dk"]);
		});

		it("should handle projects", () => {
			const res = tabCompleteActions("", { _: ["tab_completion", "start", "up", "all", ""], cwd: cwdStub });

			expect(res).toEqual(["all", "projectd", "projecte"]);
		});
	});
});

describe("Clean tab completion", () => {
	test("tabCompleteClean", () => {
		const res = tabCompleteClean({ _: ["tab_completion", "start", ""], cwd: cwdStub });

		expect(res).toEqual(["untracked", "local-changes", "master", "non-gitte", "all"]);
	});
});

describe("Toggle tab completion", () => {
	test("tabCompleteToggle", () => {
		const res = tabCompleteToggle({ _: ["tab_completion", "start", ""], cwd: cwdStub });

		expect(res).toEqual(["projecta", "projectd", "projecte", "disabledProject"]);
	});
});

describe("Switch tab completion", () => {
	test("getUppableGroupNames no-config", () => {
		const res = getUppableGroupNames(config);

		expect(res).toEqual([]);
	});

	test("getUppableGroupNames", () => {
		config.switch = { upAction: "up", downAction: "down" };
		const res = new Set(getUppableGroupNames(config));

		expect(Array.from(res)).toEqual(["cego.dk"]);
	});

	test("tabCompleteSwitch", () => {
		config.switch = { upAction: "up", downAction: "down" };
		const res = tabCompleteSwitch("", { _: ["tab_completion", "switch", ""], cwd: cwdStub });

		expect(Array.from(res)).toEqual(["cego.dk"]);
	});
});
