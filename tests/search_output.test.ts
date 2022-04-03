import chalk from "chalk";
import { Output } from "promisify-child-process";
import { searchOutputForHints } from "../src/search_output";
import { Config, SearchFor } from "../src/types/config";
import { GroupKey } from "../src/types/utils";
import { cnfStub } from "./utils/stubs";

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

		searchOutputForHints({ projects: {}, searchFor } as unknown as Config, stdoutHistory);

		expect(console.log).toHaveBeenCalledTimes(1);
		expect(console.log).toHaveBeenCalledWith(chalk`{inverse  INFO } ${searchFor[0].hint} {gray (Source: project1)}`);
	});

	test("It searches stderr", async () => {
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

		searchOutputForHints({ projects: {}, searchFor } as unknown as Config, stdoutHistory);

		expect(console.log).toHaveBeenCalledTimes(1);
		expect(console.log).toHaveBeenCalledWith(chalk`{inverse  INFO } ${searchFor[0].hint} {gray (Source: project1)}`);
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

		searchOutputForHints({ projects: {}, searchFor } as unknown as Config, stdoutHistory);

		expect(console.log).toHaveBeenCalledTimes(1);
		expect(console.log).toHaveBeenCalledWith(chalk`{inverse  INFO } ${searchFor[0].hint} {gray (Source: project1)}`);
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

		searchOutputForHints({ projects: {}, searchFor } as unknown as Config, stdoutHistory);

		expect(console.log).toHaveBeenCalledTimes(4);
		expect(console.log).toHaveBeenCalledWith(chalk`{inverse  INFO } ${searchFor[0].hint} {gray (Source: project1)}`);
		expect(console.log).toHaveBeenCalledWith(chalk`{inverse  INFO } ${searchFor[1].hint} {gray (Source: project1)}`);
		expect(console.log).toHaveBeenCalledWith(chalk`{inverse  INFO } ${searchFor[0].hint} {gray (Source: project1)}`);
		expect(console.log).toHaveBeenCalledWith(chalk`{inverse  INFO } ${searchFor[1].hint} {gray (Source: project1)}`);
	});

	test("It searches action specific searchFor", () => {
		const searchFor: SearchFor[] = [
			{
				// string contain any digit
				regex: "[0-9]",
				hint: "This string contains a digit",
			},
		];
		const stdoutHistory: (GroupKey & Output)[] = [
			{
				project: "projecta",
				action: "start",
				group: "group1",
				stdout: "This string does contain 1 digit",
				stderr: "This string does not contain a digit",
			},
		];

		const cfg: Config = JSON.parse(JSON.stringify(cnfStub));
		cfg.projects["projecta"].actions["start"].searchFor = searchFor;

		searchOutputForHints(cfg, stdoutHistory);

		expect(console.log).toHaveBeenCalledTimes(1);
		expect(console.log).toHaveBeenCalledWith(chalk`{inverse  INFO } ${searchFor[0].hint} {gray (Source: projecta)}`);
	});

	test("It supports chalking in searchFor", () => {
		const searchFor: SearchFor[] = [
			{
				// string contain any digit
				regex: "[0-9]",
				hint: "{green This string contains a digit}",
			},
		];
		const stdoutHistory: (GroupKey & Output)[] = [
			{
				project: "projecta",
				action: "start",
				group: "group1",
				stdout: "This string does contain 1 digit",
				stderr: "This string does not contain a digit",
			},
		];

		const cfg: Config = JSON.parse(JSON.stringify(cnfStub));
		cfg.projects["projecta"].actions["start"].searchFor = searchFor;

		searchOutputForHints(cfg, stdoutHistory);

		expect(console.log).toHaveBeenCalledWith(
			chalk`{inverse  INFO } {green This string contains a digit} {gray (Source: projecta)}`,
		);
	});
});
