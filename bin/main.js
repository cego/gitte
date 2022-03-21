#!/usr/bin/env node
const yargs = require("yargs/yargs");
const {hideBin} = require("yargs/helpers");
const {start} = require("../src");

const terminalWidth = yargs().terminalWidth();
yargs(hideBin(process.argv))
	.command("$0 <script> <domain>", "", (yargs) => {
		return yargs
			.positional("script", {
				describe: "script to run for each project in config",
			}).positional("domain", {
				describe: "domain for which the script is executed",
			});
	}, async (argv) => await start(argv["cwd"], argv["script"], argv["domain"]))
	.wrap(terminalWidth)
	.showHelpOnFail(false)
	.strict(true)
	.option("cwd", {
		alias: "c",
		describe: "Custom current working directory",
		type: "string",
		default: process.cwd(),
	})
	.alias("h", "help")
	.alias("v", "version")
	.argv;

