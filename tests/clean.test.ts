import { when } from "jest-when";
import fs from "fs-extra";
import * as utils from "../src/utils";
import { cnfStub } from "./utils/stubs";
import { ExecaReturnValue } from "execa";
import { GitteCleaner } from "../src/clean";

let spawnSpy: ((...args: any[]) => any) | jest.MockInstance<any, any[]>;
let promptSpy: ((...args: any[]) => any) | jest.MockInstance<any, any[]>;

beforeEach(() => {
	// @ts-ignore
	utils.spawn = jest.fn();
	console.log = jest.fn();
	console.error = jest.fn();
	fs.pathExists = jest.fn();
	fs.existsSync = jest.fn();
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

	when(fs.existsSync).mockReturnValue(true);
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
		fs.removeSync = jest.fn();
		fs.lstat = jest.fn();
		fs.readdir = jest.fn();

		// @ts-ignore
		when(fs.readdir).calledWith("/home/user/gitte").mockResolvedValue([".git", "gitte", "cego"]);

		// @ts-ignore
		when(fs.readdir).calledWith("/home/user/gitte/gitte").mockResolvedValue([]);

		// @ts-ignore
		when(fs.readdir).calledWith("/home/user/gitte/cego").mockResolvedValue(["example", "not-used"]);

		// @ts-ignore
		when(fs.readdir).calledWith("/home/user/gitte/cego/not-used").mockResolvedValue([]);

		// @ts-ignore
		when(fs.lstat).mockResolvedValue({ isDirectory: () => true });

		mockPromptNo();

		await new GitteCleaner(cnfStub).cleanNonGitte();
		expect(fs.removeSync).toHaveBeenCalledTimes(0);
		expect(fs.lstat).toHaveBeenCalledTimes(5);
		expect(fs.readdir).toHaveBeenCalledTimes(4);
	});
	it("should clean non-gitte folders", async () => {
		fs.removeSync = jest.fn();
		fs.lstat = jest.fn();
		fs.readdir = jest.fn();

		// @ts-ignore
		when(fs.readdir).calledWith("/home/user/gitte").mockResolvedValue([".git", "gitte", "cego", "logs"]);

		// @ts-ignore
		when(fs.readdir).calledWith("/home/user/gitte/gitte").mockResolvedValue([]);

		// @ts-ignore
		when(fs.readdir).calledWith("/home/user/gitte/cego").mockResolvedValue(["example", "not-used", "logs"]);

		// @ts-ignore
		when(fs.readdir).calledWith("/home/user/gitte/cego/logs").mockResolvedValue([]);

		// @ts-ignore
		when(fs.readdir).calledWith("/home/user/gitte/cego/not-used").mockResolvedValue([]);

		// @ts-ignore
		when(fs.lstat).mockResolvedValue({ isDirectory: () => true });

		mockPromptYes();

		await new GitteCleaner(cnfStub).cleanNonGitte();
		expect(fs.removeSync).toHaveBeenCalledTimes(3);
		expect(fs.removeSync).toHaveBeenCalledWith("/home/user/gitte/cego/not-used");
		expect(fs.removeSync).toHaveBeenCalledWith("/home/user/gitte/gitte");
		expect(fs.removeSync).toHaveBeenCalledWith("/home/user/gitte/cego/logs");
		expect(fs.lstat).toHaveBeenCalledTimes(7);
		expect(fs.readdir).toHaveBeenCalledTimes(5);
	});
});
