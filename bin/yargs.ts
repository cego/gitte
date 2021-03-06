#!/usr/bin/env node
import "source-map-support/register";
import yargs from "yargs/yargs";
import fs from "fs-extra";
import path from "path";

const terminalWidth = yargs().terminalWidth();
const packageJsonPath = path.join(__dirname, "../package.json");
const packageJson = JSON.parse(fs.readFileSync(packageJsonPath, "utf8"));
export const y = yargs(process.argv.slice(2))
	.env("GITTE")
	.version(packageJson["version"])
	.commandDir("cmds")
	.wrap(terminalWidth)
	.showHelpOnFail(false)
	.strict(false)
	.option("cwd", {
		alias: "c",
		describe: "Custom current working directory",
		type: "string",
		default: process.cwd(),
	})
	.option("auto-merge", {
		describe: "If on a custom branch, automatically merge default branch into current branch.",
		type: "boolean",
		default: false,
	})
	.option("needs", {
		describe: "Require dependencies of projects to be run.",
		type: "boolean",
		default: true,
	})
	.alias("h", "help");
