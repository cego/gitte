#!/usr/bin/env node
import "source-map-support/register";
import yargs from "yargs/yargs";
import fs from "fs-extra";
import path from "path";

const terminalWidth = yargs().terminalWidth();
const packageJsonPath = path.join(__dirname, "../package.json");
const packageJson = JSON.parse(fs.readFileSync(packageJsonPath, "utf8"));
yargs(process.argv.slice(2))
	.version(packageJson["version"])
	.commandDir("cmds")
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
