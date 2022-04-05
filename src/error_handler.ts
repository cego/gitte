import { AssertionError } from "assert";
import chalk from "chalk";

export function errorHandler(err: any) {
	if (err instanceof AssertionError) {
		console.error(chalk`{red ${err.message}}`);
	} else if (err.code && err.code !== 0 && err.stderr) {
		console.error(chalk`{red ${err.stderr.replace(/\n$/, "")}}`);
	} else if (err instanceof Error) {
		console.error(chalk`{red ${err.stack}}`);
	}
	if (err?.hint) {
		console.log(err.hint);
	}
	process.exit(1);
}
