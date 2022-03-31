import chalk from "chalk";
import { cnfStub, cwdStub } from "./utils/stubs";
import * as pcp from "promisify-child-process";
import { when } from "jest-when";
import { runAction, actions, fromConfig } from "../src/actions";

let spawnSpy: ((...args: any[]) => any) | jest.MockInstance<any, any[]>;
beforeEach(() => {
	// @ts-ignore
	pcp.spawn = jest.fn();
	spawnSpy = jest.spyOn(pcp, "spawn").mockResolvedValue({ stdout: "Mocked Stdout" });
	console.log = jest.fn();
	console.error = jest.fn();
});

describe("Run action", () => {
	test("Start cego.dk", async () => {
		await runAction({
			cwd: cwdStub,
			config: cnfStub,
			keys: { project: "projecta", action: "start", group: "cego.dk" },
			currentPrio: 0,
		});
		expect(console.log).toHaveBeenCalledWith(
			chalk`{blue docker-compose up} is running in {cyan /home/user/git-local-devops/cego-example}`,
		);
	});

	test("Start cego.dk, failure in script", async () => {
		when(spawnSpy)
			.calledWith("docker-compose", ["up"], expect.objectContaining({}))
			.mockRejectedValue({ stderr: "ARRRG FAILURE" });
		await runAction({
			cwd: cwdStub,
			config: cnfStub,
			keys: { project: "projecta", action: "start", group: "cego.dk" },
			currentPrio: 0,
		});
		expect(console.error).toHaveBeenCalledWith(
			chalk`"start" "cego.dk" {red failed}, goto {cyan /home/user/git-local-devops/cego-example} and run {blue docker-compose up} manually`,
		);
	});

	test("Returns if project action or group is not in config", async () => {
		// deep copy cnfStub
		const cnf = JSON.parse(JSON.stringify(cnfStub));
		cnf.projects["projecta"] = { ...cnf.projects["projecta"], actions: {} };
		cnf.projects["projectb"] = { ...cnf.projects["projecta"] };
		cnf.projects["projectb"].actions["start"] = { ...cnf.projects["projectb"].actions["start"], groups: {} };

		const res1 = await runAction({
			cwd: cwdStub,
			config: cnf,
			keys: { project: "projecta", action: "start", group: "cego.dk" },
			currentPrio: 0,
		});
		expect(res1).toBeUndefined();

		const res2 = await runAction({
			cwd: cwdStub,
			config: cnf,
			keys: { project: "projectb", action: "start", group: "cego.dk" },
			currentPrio: 0,
		});
		expect(res2).toBeUndefined();

		const res3 = await runAction({
			cwd: cwdStub,
			config: cnf,
			keys: { project: "projectc", action: "start", group: "cego.dk" },
			currentPrio: 0,
		});
		expect(res3).toBeUndefined();
	});

	test("Returns if not current prio", async () => {
		const res = await runAction({
			cwd: cwdStub,
			config: cnfStub,
			keys: { project: "projecta", action: "start", group: "cego.dk" },
			currentPrio: 1,
		});
		expect(res).toBeUndefined();
	});
});

describe("Run actions", () => {
	const keys = { project: "projecta", action: "start", group: "cego.dk" };

	test("Runs action", async () => {
		const runActionFn = jest.fn().mockResolvedValue({
			...keys,
			stdout: "Mocked Stdout",
			stderr: "Mocked Stderr",
		});

		const res = await actions(cnfStub, cwdStub, "start", "cego.dk", runActionFn);
		expect(runActionFn).toHaveBeenCalledTimes(1);
		expect(runActionFn).toHaveBeenCalledWith({
			cwd: cwdStub,
			config: cnfStub,
			keys,
			currentPrio: 0,
		});

		expect(res).toHaveLength(1);
		expect(res).toContainEqual({
			...keys,
			stdout: "Mocked Stdout",
			stderr: "Mocked Stderr",
		});
	});

	test("Runs multiple projects", async () => {
		const runActionFn = jest.fn().mockResolvedValue({
			...keys,
			stdout: "Mocked Stdout",
			stderr: "Mocked Stderr",
		});

		const cnf = { ...cnfStub };
		cnf.projects["projectb"] = { ...cnfStub.projects["projecta"] };
		cnf.projects["projectb"].priority = 1;

		cnf.projects["projectc"] = { ...cnfStub.projects["projecta"] };
		cnf.projects["projectc"].actions["start"].priority = 2;

		const res = await actions(cnfStub, cwdStub, "start", "cego.dk", runActionFn);
		expect(runActionFn).toHaveBeenCalledTimes(9); // 3 for every action because of 3 different priorities
		expect(runActionFn).toHaveBeenCalledWith({
			cwd: cwdStub,
			config: cnf,
			keys,
			currentPrio: 0,
		});
		expect(runActionFn).toHaveBeenCalledWith({
			cwd: cwdStub,
			config: cnf,
			keys: { ...keys, project: "projectb" },
			currentPrio: 1,
		});
		expect(runActionFn).toHaveBeenCalledWith({
			cwd: cwdStub,
			config: cnf,
			keys: { ...keys, project: "projectc" },
			currentPrio: 2,
		});

		expect(res).toHaveLength(9);
		expect(res).toContainEqual({
			...keys,
			stdout: "Mocked Stdout",
			stderr: "Mocked Stderr",
		});
	});
});

describe("Test fromConfig", () => {
	test("It prints hint if no action or group is found at all", async () => {
		const cnf = { ...cnfStub };
		console.log = jest.fn();

		await fromConfig(cwdStub, cnf, "nonaction", "nongroup");
		expect(console.log).toHaveBeenCalledWith(
			chalk`{yellow No groups found for action {cyan nonaction} and group {cyan nongroup}}`,
		);
	});
});
