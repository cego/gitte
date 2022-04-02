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
		});
		expect(spawnSpy).toBeCalledTimes(1);
		expect(spawnSpy).toBeCalledWith(
			"docker-compose",
			["up"],
			expect.objectContaining({ cwd: `${cwdStub}/cego-example` }),
		);
	});

	test("Start cego.dk, failure in script", async () => {
		when(spawnSpy)
			.calledWith("docker-compose", ["up"], expect.objectContaining({}))
			.mockRejectedValue({ code: "ENOENT" });
		const res = await runAction({
			cwd: cwdStub,
			config: cnfStub,
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

		const res = await actions(cnfStub, cwdStub, "start", "cego.dk", runActionFn);
		expect(runActionFn).toHaveBeenCalledTimes(1);
		expect(runActionFn).toHaveBeenCalledWith({
			cwd: cwdStub,
			config: cnfStub,
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

		const cnf = { ...cnfStub };
		cnf.projects["projectb"] = { ...cnfStub.projects["projecta"] };
		cnf.projects["projectb"].actions["start"].priority = 1;

		cnf.projects["projectc"] = { ...cnfStub.projects["projecta"] };
		cnf.projects["projectc"].actions["start"].priority = 2;

		const res = await actions(cnfStub, cwdStub, "start", "cego.dk", runActionFn);
		expect(runActionFn).toHaveBeenCalledTimes(3);
		expect(runActionFn).toHaveBeenCalledWith({
			cwd: cwdStub,
			config: cnf,
			keys,
		});
		expect(runActionFn).toHaveBeenCalledWith({
			cwd: cwdStub,
			config: cnf,
			keys: { ...keys, project: "projectb" },
		});
		expect(runActionFn).toHaveBeenCalledWith({
			cwd: cwdStub,
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
		const cnf = { ...cnfStub };
		console.log = jest.fn();

		await fromConfig(cwdStub, cnf, "nonaction", "nongroup");
		expect(console.log).toHaveBeenCalledWith(
			chalk`{yellow No groups found for action {cyan nonaction} and group {cyan nongroup}}`,
		);
	});
});
