import { cnfStub } from "./utils/stubs";
import * as utils from "../src/utils";
import { when } from "jest-when";
import {
	runAction,
	actions,
	getUniquePriorities,
	runActionPromiseWrapper,
	getProjectsToRunActionIn,
	findActionsToSkipAfterFailure,
	resolveDependenciesForActions,
	resolveProjectDependencies,
} from "../src/actions";
import { ProjectAction } from "../src/types/config";
import { GroupKey } from "../src/types/utils";
import { ActionOutputPrinter } from "../src/actions_utils";
import { ExecaReturnValue } from "execa";
import _ from "lodash";

let spawnSpy: ((...args: any[]) => any) | jest.MockInstance<any, any[]>;
const mockedActionOutputPrinter = {
	beganTask: jest.fn().mockImplementation(() => true),
	finishedTask: jest.fn(),
	init: jest.fn(),
} as unknown as ActionOutputPrinter;
let cnf: any;
beforeEach(() => {
	// deep copy cnf
	cnf = JSON.parse(JSON.stringify(cnfStub));
	// @ts-ignore
	utils.spawn = jest.fn();
	spawnSpy = jest
		.spyOn(utils, "spawn")
		.mockResolvedValue({ stdout: "Mocked Stdout" } as unknown as ExecaReturnValue<string>);
	console.log = jest.fn();
	console.error = jest.fn();
});

describe("Action", () => {
	describe("Run action", () => {
		test("Start cego.dk", async () => {
			await runAction({
				config: cnf,
				keys: { project: "projecta", action: "start", group: "cego.dk" },
				actionOutputPrinter: mockedActionOutputPrinter,
			});
			expect(spawnSpy).toBeCalledTimes(1);
			expect(spawnSpy).toBeCalledWith(
				"docker-compose",
				["up"],
				expect.objectContaining({ cwd: `${cnf.cwd}/cego/example` }),
			);
		});

		test("Start cego.dk, failure in script", async () => {
			when(spawnSpy)
				.calledWith("docker-compose", ["up"], expect.objectContaining({}))
				.mockRejectedValue({ code: "ENOENT" });
			const res = await runAction({
				config: cnf,
				keys: { project: "projecta", action: "start", group: "cego.dk" },
				actionOutputPrinter: mockedActionOutputPrinter,
			});
			expect(res.exitCode !== 0);
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

			const res = await actions(cnf, "start", "cego.dk", ["projecta"], mockedActionOutputPrinter, runActionFn);
			expect(runActionFn).toHaveBeenCalledTimes(1);
			expect(runActionFn).toHaveBeenCalledWith({
				config: cnf,
				keys,
				actionOutputPrinter: mockedActionOutputPrinter,
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

			const res = await actions(
				cnf,
				"start",
				"cego.dk",
				["projecta", "projectb", "projectc"],
				mockedActionOutputPrinter,
				runActionFn,
			);
			expect(runActionFn).toHaveBeenCalledTimes(3);
			expect(runActionFn).toHaveBeenCalledWith({
				config: cnf,
				keys,
				actionOutputPrinter: mockedActionOutputPrinter,
			});
			expect(runActionFn).toHaveBeenCalledWith({
				config: cnf,
				keys: { ...keys, project: "projectb" },
				actionOutputPrinter: mockedActionOutputPrinter,
			});
			expect(runActionFn).toHaveBeenCalledWith({
				config: cnf,
				keys: { ...keys, project: "projectc" },
				actionOutputPrinter: mockedActionOutputPrinter,
			});

			expect(res).toHaveLength(3);
			expect(res).toContainEqual({
				...keys,
				cmd: ["docker-compose", "up"],
				stdout: "Mocked Stdout",
				stderr: "Mocked Stderr",
			});
		});

		describe("Understands wildcard", () => {
			test("It should run wildcard if it is the only group match", async () => {
				cnf.projects = {
					projecta: {
						...cnf.projects.projecta,
						actions: {
							start: {
								needs: [],
								groups: {
									"*": ["docker-compose", "up"],
									"not-this": ["docker-compose", "down"],
								},
							},
						},
					},
				};

				const runActionFn = jest.fn().mockResolvedValue({ ...keys, stdout: "Mocked Stdout" });

				const res = await actions(cnf, "start", "cego.dk", ["projecta"], mockedActionOutputPrinter, runActionFn);

				expect(runActionFn).toHaveBeenCalledWith({
					config: cnf,
					keys: { ...keys, group: "*" },
					actionOutputPrinter: mockedActionOutputPrinter,
				});
				expect(runActionFn).toHaveBeenCalledTimes(1);
				expect(res).toHaveLength(1);
				expect(res).toContainEqual({
					...keys,
					stdout: "Mocked Stdout",
				});
			});

			test("It should not run wildcard if other match", async () => {
				cnf.projects = {
					projecta: {
						...cnf.projects.projecta,
						actions: {
							start: {
								groups: {
									"*": ["docker-compose", "up"],
									"not-this": ["docker-compose", "down"],
								},
							},
						},
					},
				};

				const runActionFn = jest.fn().mockResolvedValue({ ...keys, stdout: "Mocked Stdout" });

				const res = await actions(cnf, "start", "not-this", ["projecta"], mockedActionOutputPrinter, runActionFn);

				expect(runActionFn).toHaveBeenCalledWith({
					config: cnf,
					keys: { ...keys, group: "not-this" },
					actionOutputPrinter: mockedActionOutputPrinter,
				});
				expect(runActionFn).toHaveBeenCalledTimes(1);
				expect(res).toHaveLength(1);
				expect(res).toContainEqual({
					...keys,
					stdout: "Mocked Stdout",
				});
			});
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
			const actionsToRun = getProjectsToRunActionIn(cnf, "start", "cego.dk", ["projecta", "projectb", "projectc"]);
			const res = getUniquePriorities(actionsToRun);
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
				exitCode: 0,
				cmd: ["docker-compose", "up"],
			});

			const blockedActions = [
				{
					priority: null,
					searchFor: [],
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
					actionOutputPrinter: mockedActionOutputPrinter,
				},
				runActionFn,
				mockedActionOutputPrinter,
				blockedActions,
				[],
			);
			expect(runActionFn).toHaveBeenNthCalledWith(1, {
				config: cnf,
				keys,
				actionOutputPrinter: mockedActionOutputPrinter,
			});
			expect(runActionFn).toHaveBeenNthCalledWith(2, {
				config: cnf,
				keys: { ...keys, project: "projectb" },
				actionOutputPrinter: mockedActionOutputPrinter,
			});
		});
	});

	describe("getActions", () => {
		test("It finds actions", () => {
			const res = getProjectsToRunActionIn(cnf, "start", "cego.dk", ["projecta"]);
			expect(res).toHaveLength(1);
		});
		test("It solves dependency jumps", () => {
			cnf.projects = {
				projecta: {
					remote: cnf.projects["projecta"].remote,
					default_branch: cnf.projects["projecta"].default_branch,
					actions: {
						start: {
							needs: [],
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

			const res = getProjectsToRunActionIn(cnf, "start", "cego.dk", ["projecta", "projectb", "projectc"]);

			expect(res).toHaveLength(2);
			expect(res).toContainEqual(expect.objectContaining({ project: "projecta" }));
			expect(res).toContainEqual(expect.objectContaining({ project: "projectc", needs: ["projecta"] }));
		});
		describe("It resolved dependencies", () => {
			test("It resolves a project dependency", () => {
				const config = _.cloneDeep(cnf);
				const action = {
					action: "up",
					group: "cego.dk",
					project: "projectd",
					searchFor: [],
					priority: null,
					needs: ["projecte"],
					groups: {},
				};
				const res = resolveProjectDependencies(config, action);

				expect([...res]).toHaveLength(2);
				expect([...res]).toContainEqual(action);
				expect([...res]).toContainEqual(expect.objectContaining({ project: "projecte" }));
			});
			test("It resolves dependencies for multiple", () => {
				const actionsToRun: (GroupKey & ProjectAction)[] = [
					{
						action: "up",
						group: "cego.dk",
						project: "projectd",
						searchFor: [],
						priority: null,
						needs: ["projecte"],
						groups: { "cego.dk": ["docker-compose", "up"] },
					},
				];
				const config = _.cloneDeep(cnf);
				const groupToRun = "cego.dk";
				const actionToRun = "up";

				const res = resolveDependenciesForActions(actionsToRun, config, groupToRun, actionToRun);

				expect(res).toHaveLength(2);
				expect([...res]).toContainEqual(actionsToRun[0]);
				expect([...res]).toContainEqual(expect.objectContaining({ project: "projecte" }));
			});
		});
	});
	describe("findActionsToSkipAfterFailure", () => {
		test("It finds actions to skip", () => {
			const blockedActions: (GroupKey & ProjectAction)[] = [
				{
					priority: null,
					searchFor: [],
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
					priority: null,
					searchFor: [],
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
					priority: null,
					searchFor: [],
					needs: ["projecta"],
					groups: { "cego.dk": ["start"] as [string, ...string[]] },
					project: "projectb",
					action: "start",
					group: "cego.dk",
				},
				{
					priority: null,
					searchFor: [],
					needs: ["projectb"],
					groups: { "cego.dk": ["start"] as [string, ...string[]] },
					project: "projectc",
					action: "start",
					group: "cego.dk",
				},
				{
					priority: null,
					searchFor: [],
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
					priority: null,
					searchFor: [],
					needs: ["projecta"],
					groups: { "cego.dk": ["start"] as [string, ...string[]] },
					project: "projectb",
					action: "start",
					group: "cego.dk",
					wasSkippedBy: "projecta",
				},
				{
					priority: null,
					searchFor: [],
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
