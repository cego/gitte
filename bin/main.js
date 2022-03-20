#!/usr/bin/env node
const {start} = require("../src");
(async () => {
	await start(process.argv[2], process.argv[3]);
})();

