#!/usr/bin/env node
const yargs = require("yargs/yargs");
const {start} = require("../src");
const chalk = require("chalk");
const assert = require("assert");
const fs = require("fs-extra");
const path = require("path");

const terminalWidth = yargs().terminalWidth();
const packageJson = JSON.parse(fs.readFileSync(path.join(__dirname, "../package.json"), "utf8"));
yargs(process.argv.slice(2))
	.version(packageJson["version"])
	.command("$0 <action> <group>", "", (yargs) => {
		return yargs
			.positional("action", {
				describe: "action to run for each project in config",
			}).positional("group", {
				describe: "group entry to run for specified action",
			});
	}, async (argv) => {
		try {
			await start(argv["cwd"], argv["action"], argv["group"]);
		} catch (e) {
			if (e instanceof assert.AssertionError) {
				console.error(chalk`{red ${e.message}}`);
			} else if (e.message.startsWith("Process exited")) {
				const stderr = `${e["stderr"]}`.trim();
				console.error(chalk`{red ${stderr}}`);
			} else {
				console.error(chalk`{red ${e.stack}}`);
			}
			if (e.hint) console.info(e.hint);
			process.exit(1);
		}
	})
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
	.parse();
