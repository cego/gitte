import { AssertionError } from "assert";
import chalk from "chalk";
import { ErrorWithHint } from "./types/utils";

export function errorHandler(err: any) {
	let handled = false; // if the error is handled, we don't want to print stack trace
	if (err instanceof AssertionError) {
		console.error(chalk`{red ${err.message}}`);
		handled = true;
	}
	if (err.exitCode && err.exitCode !== 0) {
		if (err.stderr) console.error(chalk`{red ${err.stderr?.replace(/\n$/, "")}}`);
		if (err.stdout) console.log(chalk`${err.stdout?.replace(/\n$/, "")}`);
		handled = true;
	}
	if (err instanceof ErrorWithHint) {
		console.log(err.hint);
		handled = true;
	}

	if (!handled) {
		console.error(chalk`{red ${err.stack}}`);
	}
	process.exit(1);
}
