#!/usr/bin/env node
import "source-map-support/register";
import yargs from "yargs/yargs";
import fs from "fs-extra";
import path from "path";

const terminalWidth = yargs().terminalWidth();
const packageJsonPath = path.join(__dirname, "../package.json");
const packageJson = JSON.parse(fs.readFileSync(packageJsonPath, "utf8"));
yargs(process.argv.slice(2))
	.env("GITTE")
	.version(packageJson["version"])
	.commandDir("cmds")
	.wrap(terminalWidth)
	.showHelpOnFail(false)
	.strict(true)
	.middleware((args: any) => {
		if (args.cwd && args.cwd.startsWith(process.env.HOME)) {
			args.cwd = args.cwd.replace(process.env.HOME, "~");
		}
	})
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
	.alias("h", "help")
	.parse();
