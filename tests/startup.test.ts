import fs from "fs-extra";
import { when } from "jest-when";
import * as pcp from "promisify-child-process";
import { startup } from "../src/startup";

let spawnSpy: ((...args: any[]) => any) | jest.MockInstance<any, any[]>;
beforeEach(() => {
	// @ts-ignore
	pcp.spawn = jest.fn();
	console.log = jest.fn();
	spawnSpy = jest.spyOn(pcp, "spawn").mockResolvedValue({ stdout: "Mocked Stdout" });
	fs.pathExists = jest.fn();
});

describe("Startup checks", () => {
	test("failing argv", async () => {
		when(spawnSpy).calledWith("echo", ["hello"], expect.objectContaining({})).mockRejectedValue(new Error("WHAT"));
		await expect(startup([["test", { cmd: ["echo", "hello"] }]])).rejects.toThrow("WHAT");
	});

	test("failing shell", async () => {
		when(spawnSpy)
			.calledWith("echo hello", [], expect.objectContaining({ shell: "bash" }))
			.mockRejectedValue(new Error("WHAT"));
		await expect(startup([["test", { shell: "bash", script: "echo hello" }]])).rejects.toThrow("WHAT");
	});
});
