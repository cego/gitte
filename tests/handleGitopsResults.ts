import chalk from "chalk";
import { ErrorWithHint } from "../src/types/utils";
import { handleGitopsResults } from "../src/gitops";

beforeEach(() => {
	console.log = jest.fn();
});

describe("Print logs", () => {
	test("It logs all successful", async () => {
		const projectNames = ["test1", "test2"];
		const logs: (string | ErrorWithHint)[][] = [["log1", "log2"], ["log3"]];

		handleGitopsResults(projectNames, logs);

		expect(console.log).toHaveBeenCalledTimes(5);
		expect(console.log).toHaveBeenCalledWith(chalk`┌─ {green {bold test1}}`);
		expect(console.log).toHaveBeenCalledWith(`├─── log1`);
		expect(console.log).toHaveBeenCalledWith(`└─── log2`);
		expect(console.log).toHaveBeenCalledWith(chalk`┌─ {green {bold test2}}`);
		expect(console.log).toHaveBeenCalledWith(`└─── log3`);
	});

	test("It logs all failed", async () => {
		const projectNames = ["test1", "test2"];
		const logs: (string | ErrorWithHint)[][] = [
			[new ErrorWithHint("test error 1")],
			[new ErrorWithHint("test error 2")],
		];

		expect(() => handleGitopsResults(projectNames, logs)).toThrowError("At least one git operation failed");

		expect(console.log).toHaveBeenCalledTimes(4);
		expect(console.log).toHaveBeenCalledWith(chalk`┌─ {red {bold test1}}`);
		expect(console.log).toHaveBeenCalledWith(expect.stringContaining("test error 1"));
		expect(console.log).toHaveBeenCalledWith(chalk`┌─ {red {bold test2}}`);
		expect(console.log).toHaveBeenCalledWith(expect.stringContaining("test error 2"));
	});

	test("It logs all failed and successful", async () => {
		const projectNames = ["test1", "test2"];
		const logs: (string | ErrorWithHint)[][] = [[new ErrorWithHint("test error 1")], ["log3"]];

		expect(() => handleGitopsResults(projectNames, logs)).toThrowError("At least one git operation failed");

		expect(console.log).toHaveBeenCalledTimes(4);
		expect(console.log).toHaveBeenCalledWith(chalk`┌─ {red {bold test1}}`);
		expect(console.log).toHaveBeenCalledWith(expect.stringContaining("test error 1"));
		expect(console.log).toHaveBeenCalledWith(chalk`┌─ {green {bold test2}}`);
		expect(console.log).toHaveBeenCalledWith(`└─── log3`);
	});
});
