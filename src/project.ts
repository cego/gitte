const assert = require("assert");

function getProjectDirFromRemote(cwd, remote) {
	assert(remote.match(/git@.*?:.*?\.git/), `${remote} is not a valid project remote. Use git@gitlab.com:example/firecow.git syntax`);
	return `${cwd.replace(/\/$/, "")}/${remote.replace(/.*?:/, "").replace(/\//g, "-").replace(".git", "")}`;
}

module.exports = {getProjectDirFromRemote};
