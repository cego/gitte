import { errorHandler } from "../src/error_handler";
import { AssertionError } from "assert";
import chalk from "chalk";
import { ErrorWithHint } from "../src/types/utils";

beforeEach(() => {
	// @ts-ignore
	process.exit = jest.fn();
	console.error = jest.fn();
	console.log = jest.fn();
});

describe("Error Handler", () => {
	test("Assertion Error", async () => {
		const err = new AssertionError({ message: "Im an assertion error" });
		errorHandler(err);
		expect(console.error).toHaveBeenCalledWith(chalk`{red Im an assertion error}`);
		expect(process.exit).toHaveBeenCalledWith(1);
	});

	test("Error with hint", async () => {
		const err = new ErrorWithHint(`Have you tried turning it on and off again`, new Error("uncaught error"));
		errorHandler(err);
		expect(console.log).toHaveBeenCalledWith(chalk`Have you tried turning it on and off again`);
		expect(process.exit).toHaveBeenCalledWith(1);
	});

	test("Child process error with exit code", async () => {
		const err = new Error("child process exited");
		// @ts-ignore
		err.exitCode = 29;
		// @ts-ignore
		err.stderr = "im depressed\n";
		errorHandler(err);
		expect(console.error).toHaveBeenCalledWith(chalk`{red im depressed}`);
		expect(process.exit).toHaveBeenCalledWith(1);
	});

	test("Error", async () => {
		const err = new Error("uncaught error");
		err.stack = `Error: uncaught error\n1 error`;
		errorHandler(err);
		expect(console.error).toHaveBeenCalledWith(chalk`{red Error: uncaught error\n1 error}`);
		expect(process.exit).toHaveBeenCalledWith(1);
	});
});
