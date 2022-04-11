import { ActionOutputPrinter } from "../src/actions_utils";
import * as utils from "../src/utils";
import { ExecaReturnValue } from "execa";
import { cnfStub } from "./utils/stubs";
import fs from "fs-extra";

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
			.mockResolvedValue({ stdout: "Mocked Stdout" } as unknown as ExecaReturnValue<string>);

		const actionOutputPrinter = new ActionOutputPrinter(cnfStub, "start", "cego.dk", "projecta");
		await actionOutputPrinter.run();

		expect(spawnSpy).toBeCalledTimes(1);
		expect(fs.writeFile).toBeCalledTimes(1);
		expect(process.stdout.write).toBeCalledTimes(3);
	});
});
