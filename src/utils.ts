import chalk from "chalk";
import { AssertionError } from "assert";
import { ErrorWithHint } from "./types/utils";
import execa from "execa";
import { string } from "yargs";

export function printLogs(projectNames: string[], logs: (string | ErrorWithHint)[][]) {
	let errorCount = 0;
	for (const [i, projectName] of projectNames.entries()) {
		const isError = logs[i].filter((log) => log instanceof ErrorWithHint).length > 0;

		if (!isError) {
			console.log(chalk`┌─ {green {bold ${projectName}}}`);
		} else {
			errorCount++;
			console.log(chalk`┌─ {red {bold ${projectName}}}`);
		}

		for (const [j, log] of logs[i].entries()) {
			let formattedLog = "";
			if (log instanceof ErrorWithHint) {
				formattedLog = chalk`{red ${log.message}}`;
			} else {
				formattedLog = log;
			}

			console.log(`${j === logs[i].length - 1 ? "└" : "├"}─── ${formattedLog}`);
		}
	}
	if (errorCount > 0) {
		throw new AssertionError({ message: "At least one git operation failed" });
	}
}

export function printHeader(header: string) {
	console.log();
	console.log(chalk`{bgCyan  BEGIN } {bold ${header}}`);
	console.log();
}

export function spawn(file: string, args?: string[], options?: execa.Options): execa.ExecaChildProcess {
	return execa(file, args, options);
}
