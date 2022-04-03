import chalk from "chalk";
import { cnfStub } from "./utils/stubs";
import * as pcp from "promisify-child-process";
import { when } from "jest-when";
import {
	runAction,
	actions,
	fromConfig,
	getUniquePriorities,
	runActionPromiseWrapper,
	getActions,
	findActionsToSkipAfterFailure,
} from "../src/actions";
import { SingleBar } from "cli-progress";
import { Config, ProjectAction } from "../src/types/config";
import { GroupKey } from "../src/types/utils";
import fs from "fs-extra";

let spawnSpy: ((...args: any[]) => any) | jest.MockInstance<any, any[]>;
let cnf: Config;
beforeEach(() => {
	// deep copy cnf
	cnf = JSON.parse(JSON.stringify(cnfStub));
	// @ts-ignore
	pcp.spawn = jest.fn();
	spawnSpy = jest.spyOn(pcp, "spawn").mockResolvedValue({ stdout: "Mocked Stdout" });
	console.log = jest.fn();
	console.error = jest.fn();
});

describe("Action", () => {
	describe("Run action", () => {
		test("Start cego.dk", async () => {
			await runAction({
				config: cnf,
				keys: { project: "projecta", action: "start", group: "cego.dk" },
			});
			expect(spawnSpy).toBeCalledTimes(1);
			expect(spawnSpy).toBeCalledWith(
				"docker-compose",
				["up"],
				expect.objectContaining({ cwd: `${cnf.cwd}/cego-example` }),
			);
		});

		test("Start cego.dk, failure in script", async () => {
			when(spawnSpy)
				.calledWith("docker-compose", ["up"], expect.objectContaining({}))
				.mockRejectedValue({ code: "ENOENT" });
			const res = await runAction({
				config: cnf,
				keys: { project: "projecta", action: "start", group: "cego.dk" },
			});
			expect(res.code !== 0);
		});
	});

	describe("Run actions", () => {
		const keys = { project: "projecta", action: "start", group: "cego.dk" };

		test("Runs action", async () => {
			const runActionFn = jest.fn().mockResolvedValue({
				...keys,
				stdout: "Mocked Stdout",
				stderr: "Mocked Stderr",
				cmd: ["docker-compose", "up"],
			});

			const res = await actions(cnf, "start", "cego.dk", runActionFn);
			expect(runActionFn).toHaveBeenCalledTimes(1);
			expect(runActionFn).toHaveBeenCalledWith({
				config: cnf,
				keys,
			});

			expect(res).toHaveLength(1);
			expect(res).toContainEqual({
				...keys,
				stdout: "Mocked Stdout",
				stderr: "Mocked Stderr",
				cmd: ["docker-compose", "up"],
			});
		});

		test("Runs multiple projects", async () => {
			const runActionFn = jest.fn().mockResolvedValue({
				...keys,
				stdout: "Mocked Stdout",
				stderr: "Mocked Stderr",
				cmd: ["docker-compose", "up"],
			});

			cnf.projects["projectb"] = { ...cnf.projects["projecta"] };
			cnf.projects["projectb"].actions["start"].priority = 1;

			cnf.projects["projectc"] = { ...cnf.projects["projecta"] };
			cnf.projects["projectc"].actions["start"].priority = 2;

			const res = await actions(cnf, "start", "cego.dk", runActionFn);
			expect(runActionFn).toHaveBeenCalledTimes(3);
			expect(runActionFn).toHaveBeenCalledWith({
				config: cnf,
				keys,
			});
			expect(runActionFn).toHaveBeenCalledWith({
				config: cnf,
				keys: { ...keys, project: "projectb" },
			});
			expect(runActionFn).toHaveBeenCalledWith({
				config: cnf,
				keys: { ...keys, project: "projectc" },
			});

			expect(res).toHaveLength(3);
			expect(res).toContainEqual({
				...keys,
				cmd: ["docker-compose", "up"],
				stdout: "Mocked Stdout",
				stderr: "Mocked Stderr",
			});
		});
	});

	describe("Test fromConfig", () => {
		test("It prints hint if no action or group is found at all", async () => {
			fs.writeFileSync = jest.fn();
			await fromConfig(cnf, "nonaction", "nongroup");
			expect(console.log).toHaveBeenCalledWith(
				chalk`{yellow No groups found for action {cyan nonaction} and group {cyan nongroup}}`,
			);
		});
	});

	describe("getUniquePriorities", () => {
		test("It returns unique priorities", () => {
			cnf.projects["projectb"] = { ...cnf.projects["projecta"] };
			cnf.projects["projectb"].actions["start"].priority = 2;

			// deep copy project a
			cnf.projects["projectc"] = JSON.parse(JSON.stringify(cnf.projects["projecta"]));
			cnf.projects["projectc"].actions["start"].priority = 1;

			// deep copy project a
			cnf.projects["projectd"] = JSON.parse(JSON.stringify(cnf.projects["projecta"]));
			cnf.projects["projectd"].actions["start"].priority = 2;

			const res = getUniquePriorities(cnf, "start", "cego.dk");
			expect(res).toEqual(new Set([1, 2]));
		});
	});

	describe("runActionPromiseWrapper", () => {
		test("It recursively calls itself when needs resolve", async () => {
			const keys = { project: "projecta", action: "start", group: "cego.dk" };

			const runActionFn = jest.fn().mockResolvedValue({
				...keys,
				stdout: "Mocked Stdout",
				stderr: "Mocked Stderr",
				code: 0,
				cmd: ["docker-compose", "up"],
			});
			const progressBarMock = {
				update: jest.fn(),
				increment: jest.fn(),
			} as unknown as SingleBar;

			const blockedActions = [
				{
					needs: ["projecta"],
					groups: { "cego.dk": ["start"] as [string, ...string[]] },
					...keys,
					project: "projectb",
				},
			];

			await runActionPromiseWrapper(
				{
					config: cnf,
					keys,
				},
				runActionFn,
				progressBarMock,
				blockedActions,
				[],
			);
			expect(runActionFn).toHaveBeenNthCalledWith(1, {
				config: cnf,
				keys,
			});
			expect(runActionFn).toHaveBeenNthCalledWith(2, {
				config: cnf,
				keys: { ...keys, project: "projectb" },
			});
		});
	});

	describe("getActions", () => {
		test("It finds actions", () => {
			const res = getActions(cnf, "start", "cego.dk");
			expect(res).toHaveLength(1);
		});
		test("It solves dependency jumps", () => {
			cnf.projects = {
				projecta: {
					remote: cnf.projects["projecta"].remote,
					default_branch: cnf.projects["projecta"].default_branch,
					actions: {
						start: {
							groups: { "cego.dk": ["start"] },
						},
					},
				},
				projectb: {
					remote: cnf.projects["projecta"].remote,
					default_branch: cnf.projects["projecta"].default_branch,
					actions: {
						start: {
							groups: {},
							needs: ["projecta"],
						},
					},
				},
				projectc: {
					remote: cnf.projects["projecta"].remote,
					default_branch: cnf.projects["projecta"].default_branch,
					actions: {
						start: {
							groups: { "cego.dk": ["start"] },
							needs: ["projectb"],
						},
					},
				},
			};

			const res = getActions(cnf, "start", "cego.dk");

			expect(res).toHaveLength(2);
			expect(res).toContainEqual(expect.objectContaining({ project: "projecta" }));
			expect(res).toContainEqual(expect.objectContaining({ project: "projectc", needs: ["projecta"] }));
		});
	});
	describe("findActionsToSkipAfterFailure", () => {
		test("It finds actions to skip", () => {
			const blockedActions: (GroupKey & ProjectAction)[] = [
				{
					needs: ["projecta"],
					groups: { "cego.dk": ["start"] as [string, ...string[]] },
					project: "projectb",
					action: "start",
					group: "cego.dk",
				},
			];
			const res = findActionsToSkipAfterFailure("projecta", blockedActions);

			expect(res).toEqual([
				{
					needs: ["projecta"],
					groups: { "cego.dk": ["start"] },
					project: "projectb",
					action: "start",
					group: "cego.dk",
					wasSkippedBy: "projecta",
				},
			]);
		});

		test("It finds chained actions to skip", () => {
			const blockedActions: (GroupKey & ProjectAction)[] = [
				{
					needs: ["projecta"],
					groups: { "cego.dk": ["start"] as [string, ...string[]] },
					project: "projectb",
					action: "start",
					group: "cego.dk",
				},
				{
					needs: ["projectb"],
					groups: { "cego.dk": ["start"] as [string, ...string[]] },
					project: "projectc",
					action: "start",
					group: "cego.dk",
				},
				{
					needs: ["projectd"],
					groups: { "cego.dk": ["start"] as [string, ...string[]] },
					project: "projecte",
					action: "start",
					group: "cego.dk",
				},
			];
			const res = findActionsToSkipAfterFailure("projecta", blockedActions);

			expect(res).toEqual([
				{
					needs: ["projecta"],
					groups: { "cego.dk": ["start"] as [string, ...string[]] },
					project: "projectb",
					action: "start",
					group: "cego.dk",
					wasSkippedBy: "projecta",
				},
				{
					needs: ["projectb"],
					groups: { "cego.dk": ["start"] as [string, ...string[]] },
					project: "projectc",
					action: "start",
					group: "cego.dk",
					wasSkippedBy: "projectb",
				},
			]);
			expect(blockedActions.filter((x) => x)).toHaveLength(1);
		});
	});
});
