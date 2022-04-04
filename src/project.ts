import assert from "assert";

export function getProjectDirFromRemote(cwd: string, remote: string) {
	const match = remote.match(/git@.*?:.*?\.git/);
	assert(match, `${remote} is not a valid project remote. Use git@gitlab.com:example/cego.git syntax`);
	return `${cwd.replace(/\/$/, "")}/${remote.replace(/.*?:/, "").replace(".git", "")}`;
}
