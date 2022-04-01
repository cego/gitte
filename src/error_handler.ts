import {AssertionError} from "assert";
import chalk from "chalk";

export function errorHandler(err: unknown) {
    if (err instanceof AssertionError) {
        console.error(chalk`{red ${err.message}}`);
    } else if (err instanceof Error) {
        console.error(chalk`{red ${err.stack}}`);
    }
    process.exit(1)
}