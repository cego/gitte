import assert from "assert";

export function getProjectDirFromRemote(cwd: string, remote: string) {
	assert(remote.match(/git@.*?:.*?\.git/), `${remote} is not a valid project remote. Use git@gitlab.com:example/cego.git syntax`);
	return `${cwd.replace(/\/$/, "")}/${remote.replace(/.*?:/, "").replace(/\//g, "-").replace(".git", "")}`;
}
