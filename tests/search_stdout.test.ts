import chalk from "chalk";
import { Output } from "promisify-child-process";
import { searchStdoutAndPrintHints } from "../src/search_stdout";
import { SearchFor } from "../src/types/config";
import { GroupKey } from "../src/types/utils";

beforeEach(() => {
	console.log = jest.fn();
});

describe("Search action output", () => {
	test("It searches stdout", async () => {
		const searchFor: SearchFor[] = [
			{
				// string contain any digit
				regex: "[0-9]",
				hint: "This string contains a digit",
			},
		];
		const stdoutHistory: (GroupKey & Output)[] = [
			{
				project: "project1",
				action: "action1",
				group: "group1",
				stdout: "This string does contain 1 digit",
				stderr: "This string does not contain a digit",
			},
		];

		searchStdoutAndPrintHints(searchFor, stdoutHistory);

		expect(console.log).toHaveBeenCalledTimes(1);
		expect(console.log).toHaveBeenCalledWith(
			chalk`{yellow Hint: ${searchFor[0].hint}} {gray (Source: project1/action1/group1)}`,
		);
	});

	test("It searches stdout", async () => {
		const searchFor: SearchFor[] = [
			{
				// string contain any digit
				regex: "[0-9]",
				hint: "This string contains a digit",
			},
		];
		const stdoutHistory: (GroupKey & Output)[] = [
			{
				project: "project1",
				action: "action1",
				group: "group1",
				stdout: "This string does contain 1 digit",
				stderr: "This string does not contain a digit",
			},
		];

		searchStdoutAndPrintHints(searchFor, stdoutHistory);

		expect(console.log).toHaveBeenCalledTimes(1);
		expect(console.log).toHaveBeenCalledWith(
			chalk`{yellow Hint: ${searchFor[0].hint}} {gray (Source: project1/action1/group1)}`,
		);
	});

	test("It searches stdout and stderr", async () => {
		const searchFor: SearchFor[] = [
			{
				// string contain any digit
				regex: "[0-9]",
				hint: "This string contains a digit",
			},
		];
		const stdoutHistory: (GroupKey & Output)[] = [
			{
				project: "project1",
				action: "action1",
				group: "group1",
				stdout: "This string does contain 1 digit",
				stderr: "This string does contain 1 digit",
			},
		];

		searchStdoutAndPrintHints(searchFor, stdoutHistory);

		expect(console.log).toHaveBeenCalledTimes(1);
		expect(console.log).toHaveBeenCalledWith(
			chalk`{yellow Hint: ${searchFor[0].hint}} {gray (Source: project1/action1/group1)}`,
		);
	});

	test("It searches multiple outputs", async () => {
		const searchFor: SearchFor[] = [
			{
				// string contain any digit
				regex: "[0-9]",
				hint: "This string contains a digit",
			},
			{
				// string contain any letter
				regex: "[a-z]",
				hint: "This string contains a letter",
			},
		];
		const stdoutHistory: (GroupKey & Output)[] = [
			{
				project: "project1",
				action: "action1",
				group: "group1",
				stdout: "1",
				stderr: "",
			},
			{
				project: "project1",
				action: "action1",
				group: "group2",
				stdout: "letter",
				stderr: "",
			},
			{
				project: "project1",
				action: "action1",
				group: "group3",
				stdout: "@!???",
				stderr: ":) 🤷‍♂️",
			},
			{
				project: "project1",
				action: "action1",
				group: "group4",
				stdout: "letter 123",
				stderr: "",
			},
		];

		searchStdoutAndPrintHints(searchFor, stdoutHistory);

		expect(console.log).toHaveBeenCalledTimes(4);
		expect(console.log).toHaveBeenCalledWith(
			chalk`{yellow Hint: ${searchFor[0].hint}} {gray (Source: project1/action1/group1)}`,
		);
		expect(console.log).toHaveBeenCalledWith(
			chalk`{yellow Hint: ${searchFor[1].hint}} {gray (Source: project1/action1/group2)}`,
		);
		expect(console.log).toHaveBeenCalledWith(
			chalk`{yellow Hint: ${searchFor[0].hint}} {gray (Source: project1/action1/group4)}`,
		);
		expect(console.log).toHaveBeenCalledWith(
			chalk`{yellow Hint: ${searchFor[1].hint}} {gray (Source: project1/action1/group4)}`,
		);
	});
});
