import { getProjectDirFromRemote } from "../src/project";
import { cwdStub } from "./utils/stubs";

describe("Project dir from remote", () => {
	test("Valid ssh remote", () => {
		const dir = getProjectDirFromRemote(
			cwdStub,
			"git@gitlab.com:cego/example.git",
		);
		expect(dir).toEqual(`${cwdStub}/cego-example`);
	});

	test("Valid ssh remote with cwd ending in slash", () => {
		const dir = getProjectDirFromRemote(
			`${cwdStub}/`,
			"git@gitlab.com:cego/example.git",
		);
		expect(dir).toEqual(`${cwdStub}/cego-example`);
	});

	test("Invalid remote", () => {
		expect(() => {
			getProjectDirFromRemote(
				cwdStub,
				"git@gitlab.coinvalidirecow/example.git",
			);
		}).toThrowError(
			"git@gitlab.coinvalidirecow/example.git is not a valid project remote. Use git@gitlab.com:example/cego.git syntax",
		);
	});
});
