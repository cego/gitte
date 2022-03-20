const {getProjectDirFromRemote} = require("../src/project");

describe("Project dir from remote", () => {

	test("valid ssh remote", () => {
		const dir = getProjectDirFromRemote("/home/user/git-local-devops", "git@gitlab.com:firecow/example.git");
		expect(dir).toEqual("/home/user/git-local-devops/firecow/example");
	});

	test("valid ssh remote with cwd ending in slash", () => {
		const dir = getProjectDirFromRemote("/home/user/git-local-devops/", "git@gitlab.com:firecow/example.git");
		expect(dir).toEqual("/home/user/git-local-devops/firecow/example");
	});

	test("invvalid remote", () => {
		expect(() => {
			getProjectDirFromRemote("/home/user/git-local-devops/", "git@gitlab.coinvalidirecow/example.git");
		}).toThrowError("git@gitlab.coinvalidirecow/example.git is not a valid project remote. Use git@gitlab.com:example/firecow.git syntax");
	});
});

