const cp = require("promisify-child-process");
const {default: to} = require("await-to-js");

async function startup(startupList) {
	let err;
	for (const entry of startupList) {
		const argv = entry["argv"];
		if (argv) {
			[err] = await to(cp.spawn(argv[0], argv.slice(1), {env: process.env, encoding: "utf8"}));
			if (err) {
				err.hint = entry["hint"];
				throw err;
			}
		} else {
			[err] = await to(cp.spawn(entry["script"], {shell: entry["shell"], env: process.env, encoding: "utf8"}));
			if (err) {
				err.hint = entry["hint"];
				throw err;
			}
		}
	}
}

module.exports = {startup};
