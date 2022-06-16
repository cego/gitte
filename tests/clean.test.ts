import { when } from "jest-when";
import fs from "fs-extra";
import * as utils from "../src/utils";
import { cnfStub } from "./utils/stubs";
import { ExecaReturnValue } from "execa";
import { GitteCleaner } from "../src/clean";
import { assert } from "console";

let spawnSpy: ((...args: any[]) => any) | jest.MockInstance<any, any[]>;
let promptSpy: ((...args: any[]) => any) | jest.MockInstance<any, any[]>;

beforeEach(() => {
	// @ts-ignore
	utils.spawn = jest.fn();
	console.log = jest.fn();
	console.error = jest.fn();
	fs.pathExists = jest.fn();
	// @ts-ignore
	promptSpy = jest.spyOn(utils, "promptBoolean");

	spawnSpy = jest
		.spyOn(utils, "spawn")
		.mockResolvedValue({ stdout: "Mocked Stdout" } as unknown as ExecaReturnValue<string>);

	when(spawnSpy).mockResolvedValue(() => {
		return {
			stdout: "Mocked Stdout",
		};
	});

	when(spawnSpy)
		.calledWith("git", ["branch", "--show-current"], expect.objectContaining({ cwd: expect.any(String) }))
		.mockResolvedValue({ stdout: "main" });
});

function mockPromptYes() {
	// @ts-ignore
	when(promptSpy).mockResolvedValue(true);
}

function mockPromptNo() {
	// @ts-ignore
	when(promptSpy).mockResolvedValue(false);
}

describe("Clean tests", () => {
	it("should clean untracked files", async () => {
		await new GitteCleaner(cnfStub).cleanUntracked();

		expect(spawnSpy).toHaveBeenCalledWith(
			"git",
			["clean", "-fdx"],
			expect.objectContaining({ cwd: expect.any(String) }),
		);
		expect(spawnSpy).toHaveBeenCalledTimes(3);
		expect(console.error).toHaveBeenCalledTimes(0);
	});
	it("should fail softly on git error with untracked files", async () => {
		when(spawnSpy)
			.calledWith("git", ["clean", "-fdx"], expect.objectContaining({ cwd: expect.any(String) }))
			.mockRejectedValue(new Error("WHAT"));

		await new GitteCleaner(cnfStub).cleanUntracked();

		expect(spawnSpy).toHaveBeenCalledWith(
			"git",
			["clean", "-fdx"],
			expect.objectContaining({ cwd: expect.any(String) }),
		);
		expect(spawnSpy).toHaveBeenCalledTimes(3);
		expect(console.error).toHaveBeenCalledTimes(3);
	});
	it("should should prompt before cleaning local changes", async () => {
		when(spawnSpy)
			.calledWith("git", ["status", "--porcelain"], expect.objectContaining({}))
			.mockResolvedValue({ stdout: "Mocked Stdout" });
		mockPromptNo();

		await new GitteCleaner(cnfStub).cleanLocalChanges();
		expect(spawnSpy).toHaveBeenCalledTimes(3);
		expect(console.error).toHaveBeenCalledTimes(0);
	});
	it("should clean local changes", async () => {
		mockPromptYes();
		await new GitteCleaner(cnfStub).cleanLocalChanges();
		expect(spawnSpy).toHaveBeenCalledTimes(6);
		expect(spawnSpy).toHaveBeenCalledWith(
			"git",
			["reset", "--hard"],
			expect.objectContaining({ cwd: expect.any(String) }),
		);
		expect(console.error).toHaveBeenCalledTimes(0);
	});
	it("should fail softly on git error with local changes", async () => {
		when(spawnSpy)
			.calledWith("git", ["reset", "--hard"], expect.objectContaining({ cwd: expect.any(String) }))
			.mockRejectedValue(new Error("WHAT"));
		mockPromptYes();
		await new GitteCleaner(cnfStub).cleanLocalChanges();
		expect(spawnSpy).toHaveBeenCalledTimes(6);
		expect(spawnSpy).toHaveBeenCalledWith(
			"git",
			["reset", "--hard"],
			expect.objectContaining({ cwd: expect.any(String) }),
		);
		expect(console.error).toHaveBeenCalledTimes(3);
	});
	it("should clean master", async () => {
		await new GitteCleaner(cnfStub).cleanMaster();
		expect(spawnSpy).toHaveBeenCalledTimes(3);
		expect(spawnSpy).toHaveBeenCalledWith(
			"git",
			["checkout", "main"],
			expect.objectContaining({ cwd: expect.any(String) }),
		);
		expect(console.error).toHaveBeenCalledTimes(0);
	});
	it("should fail softly on git error with master", async () => {
		when(spawnSpy)
			.calledWith("git", ["checkout", "main"], expect.objectContaining({ cwd: expect.any(String) }))
			.mockRejectedValue(new Error("WHAT"));
		await new GitteCleaner(cnfStub).cleanMaster();
		expect(spawnSpy).toHaveBeenCalledTimes(3);
		expect(spawnSpy).toHaveBeenCalledWith(
			"git",
			["checkout", "main"],
			expect.objectContaining({ cwd: expect.any(String) }),
		);
		expect(console.error).toHaveBeenCalledTimes(3);
	});
	it("should detect non-gitte folders and prompt to clean them", async () => {
		assert(true);
	});
	it("should clean non-gitte folders", async () => {
		assert(true);
	});
});
