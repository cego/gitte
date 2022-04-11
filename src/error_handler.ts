import { AssertionError } from "assert";
import chalk from "chalk";
import { ErrorWithHint } from "./types/utils";

export function errorHandler(err: any) {
	if (err instanceof AssertionError) {
		console.error(chalk`{red ${err.message}}`);
	} else if (err.exitCode && err.exitCode !== 0) {
		if (err.stderr) console.error(chalk`{red ${err.stderr?.replace(/\n$/, "")}}`);
		if (err.stdout) console.log(chalk`${err.stdout?.replace(/\n$/, "")}`);
	}

	if (err instanceof ErrorWithHint) {
		console.log(err.hint);
	} else {
		console.error(chalk`{red ${err.stack}}`);
	}
	process.exit(1);
}
