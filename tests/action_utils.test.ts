import { ActionOutputPrinter } from "../src/actions_utils";
import * as utils from "../src/utils";
import { ExecaReturnValue } from "execa";
import { cnfStub } from "./utils/stubs";
import fs from "fs-extra";
import { ChildProcessOutput, GroupKey } from "../src/types/utils";

describe("ActionOutputPrinter", () => {
	test("It runs actions and prints output", async () => {
		// @ts-ignore
		utils.spawn = jest.fn();
		fs.writeFile = jest.fn();
		fs.mkdir = jest.fn();
		fs.pathExists = jest.fn().mockImplementation(() => Promise.resolve(true));
		console.error = jest.fn();
		console.log = jest.fn();
		process.stdout.write = jest.fn();
		const spawnSpy = jest
			.spyOn(utils, "spawn")
			.mockResolvedValue({ stdout: "Mocked Stdout", exitCode: 0 } as unknown as ExecaReturnValue<string>);

		const actionOutputPrinter = new ActionOutputPrinter(cnfStub, "start", "cego.dk", "projecta");
		await actionOutputPrinter.run();

		expect(spawnSpy).toBeCalledTimes(1);
		expect(fs.writeFile).toBeCalledTimes(1);
		expect(process.stdout.write).toBeCalledTimes(3);
	});

	test("It stashes logs to files", async () => {
		const actionOutputPrinter = new ActionOutputPrinter(cnfStub, "start", "cego.dk", "projecta");
		const logs: (GroupKey & ChildProcessOutput)[] = [
			{
				action: "start",
				group: "group1",
				project: "projecta",
				stdout: "stdout1\nstdout2",
				stderr: "stderr1\nstderr2",
				exitCode: 128,
				cmd: ["ls", "-la"],
			},
			{
				action: "start",
				group: "group2",
				project: "projectb",
				stdout: "stdout3\nstdout4",
				stderr: "stderr3\nstderr4",
				exitCode: 0,
				cmd: ["ls", "-la"],
			},
		];

		fs.pathExists = jest.fn().mockImplementation(() => Promise.resolve(true));
		fs.writeFile = jest.fn();

		await actionOutputPrinter.stashLogsToFile(logs);
		expect(fs.writeFile).toHaveBeenCalledTimes(2);
	});

	describe("Parse run keys", () => {
		test("It parses multiple keys", () => {
			const actionOutputPrinter = new ActionOutputPrinter(cnfStub, "start", "cego.dk", "projecta");
			const keys = actionOutputPrinter.parseRunKeys("up+down", "cego.dk+example.dk", "projecta+projectb");
			expect(keys).toEqual([
				["up", "down"],
				["cego.dk", "example.dk"],
				["projecta", "projectb"],
			]);
		});

		test("It parses star key", () => {
			const actionOutputPrinter = new ActionOutputPrinter(cnfStub, "start", "cego.dk", "projecta");
			const keys = actionOutputPrinter.parseRunKeys("*", "*", "*");
			expect(keys).toEqual([
				["start", "down", "up"],
				["cego.dk", "example.com"],
				["projecta", "projectd", "projecte"],
			]);
		});

		test("It parses single keys", () => {
			const actionOutputPrinter = new ActionOutputPrinter(cnfStub, "start", "cego.dk", "projecta");
			const keys = actionOutputPrinter.parseRunKeys("action", "group", "projecta");
			expect(keys).toEqual([["action"], ["group"], ["projecta"]]);
		});
	});
});
