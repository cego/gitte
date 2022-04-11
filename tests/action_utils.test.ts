// import { ActionOutputPrinter } from "../src/actions_utils";
// import * as utils from "../src/utils";
// import { ExecaReturnValue } from "execa";
// import { cnfStub } from "./utils/stubs";
// import fs from "fs-extra";

// describe("ActionOutputPrinter", () => {
// 	test("It runs actions and prints output", async () => {
// 		// @ts-ignore
// 		utils.spawn = jest.fn();
// 		fs.writeFileSync = jest.fn();
// 		console.error = jest.fn();
// 		console.log = jest.fn();
// 		process.stdout.write = jest.fn();
// 		const spawnSpy = jest
// 			.spyOn(utils, "spawn")
// 			.mockResolvedValue({ stdout: "Mocked Stdout" } as unknown as ExecaReturnValue<string>);

// 		const actionOutputPrinter = new ActionOutputPrinter(cnfStub, "start", "cego.dk");
// 		await actionOutputPrinter.run();

// 		expect(spawnSpy).toBeCalledTimes(1);
// 		expect(fs.writeFileSync).toBeCalledTimes(1);
// 		expect(process.stdout.write).toBeCalledTimes(3);
// 	});
// });
