import fs from "fs-extra";
import { when } from "jest-when";
import * as utils from "../src/utils";
import { startup } from "../src/startup";
import { Config } from "../src/types/config";
import { cwdStub } from "./utils/stubs";
import { ExecaReturnValue } from "execa";

let spawnSpy: ((...args: any[]) => any) | jest.MockInstance<any, any[]>;
beforeEach(() => {
	// @ts-ignore
	utils.spawn = jest.fn();
	console.log = jest.fn();
	spawnSpy = jest
		.spyOn(utils, "spawn")
		.mockResolvedValue({ stdout: "Mocked Stdout" } as unknown as ExecaReturnValue<string>);
	fs.pathExists = jest.fn();
});

describe("Startup checks", () => {
	test("failing argv", async () => {
		when(spawnSpy).calledWith("echo", ["hello"], expect.objectContaining({})).mockRejectedValue(new Error("WHAT"));
		await expect(
			startup({ cwd: cwdStub, startup: { test: { cmd: ["echo", "hello"] } } } as unknown as Config),
		).rejects.toThrow("WHAT");
	});

	test("failing shell", async () => {
		when(spawnSpy)
			.calledWith("echo hello", [], expect.objectContaining({ shell: "bash" }))
			.mockRejectedValue(new Error("WHAT"));
		await expect(
			startup({ cwd: cwdStub, startup: { test: { shell: "bash", script: "echo hello" } } } as unknown as Config),
		).rejects.toThrow("WHAT");
	});
});
