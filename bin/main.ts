#!/usr/bin/env node
import yargs from 'yargs/yargs';
import { start } from "../src";
import chalk from "chalk";
import assert from "assert";
import fs from "fs-extra";
import path from "path";
import { loadConfig } from '../src/config_loader';

const terminalWidth = yargs().terminalWidth();
const packageJson = JSON.parse(fs.readFileSync(path.join(__dirname, "../package.json"), "utf8"));
yargs(process.argv.slice(2))
	.version(packageJson["version"])
	.command({
		handler: async (argv) => {
			try {
				if(argv.list){
					const config = await loadConfig(argv.cwd as string);
					for(const [name, project] of Object.entries(config.projects)){
						console.log(chalk`{bold ${name}:} {cyan [${Object.keys(project.actions).join(", ")}]}`)
					}
					return
				}
				if(argv.validate){
					await loadConfig(argv.cwd as string);
					console.log(chalk`{green .git-local-devops.yml is valid}`)
					return;
				}

				assert(argv.action && argv.group, "Missing required positional parameters: action and group are required, see --help for more info.");
				
	
				await start(argv.cwd as string, argv.action as string, argv.group as string);
			} catch (e: any) {
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
		},
		builder: (y) => {
			return y
				.positional("action", {
					required: false,
					describe: "action to run for each project in config",
				}).positional("group", {
					required: false,
					describe: "group entry to run for specified action",
				});
			},
		describe: "run action for a project in config",
		command: "$0 [action] [group]",
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
	.option("list", {
		alias: "l",
		describe: "List all projects and their actions",
		type: "boolean",
		default: false
		
	})
	.option("validate", {
		describe: "Validate the configuration",
		type: "boolean",
		default: false
	})
	.alias("h", "help")
	.parse();
