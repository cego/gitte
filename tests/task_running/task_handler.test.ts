import { TaskHandler } from "../../src/task_running/task_handler";
import { cnfStub } from "../utils/stubs";
import * as utils from "../../src/utils";
import fs from "fs-extra";
import { ExecaReturnValue } from "execa";

describe("Task Handler tests", () => {
	it("runs and prints output", async () => {
		// @ts-ignore
		fs.writeFileSync = jest.fn();
		fs.ensureFileSync = jest.fn();
		fs.pathExists = jest.fn().mockImplementation(() => Promise.resolve(true));
		console.error = jest.fn();
		console.log = jest.fn();
		process.stdout.write = jest.fn();
		const spawnSpy = jest
			.spyOn(utils, "spawn")
			.mockResolvedValue({ stdout: "Mocked Stdout", exitCode: 0 } as unknown as ExecaReturnValue<string>);

		const taskHandler = new TaskHandler(cnfStub, "start", "cego.dk", "projecta", 1);
		await taskHandler.run();

		expect(spawnSpy).toBeCalledTimes(1);
		expect(fs.writeFileSync).toBeCalledTimes(1);
		expect(process.stdout.write).toBeCalledTimes(3);
	});
});
