import { errorHandler } from "../src/error_handler";
import { AssertionError } from "assert";
import chalk from "chalk";

beforeEach(() => {
	// @ts-ignore
	process.exit = jest.fn();
	console.error = jest.fn();
});

describe("Error Handler", () => {
	test("Assertion Error", async () => {
		const err = new AssertionError({ message: "Im an assertion error" });
		errorHandler(err);
		expect(console.error).toHaveBeenCalledWith(chalk`{red Im an assertion error}`);
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
