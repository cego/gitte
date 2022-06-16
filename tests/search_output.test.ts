import { getLogFilePath, sortTasksByTimeThenState, stashLogsToFile } from "../src/search_output";
import { cnfStub, cwdStub, getTask } from "./utils/stubs";
import fs from "fs-extra";
import { TaskState } from "../src/task_running/task";
import assert from "assert";

beforeEach(() => {
	console.log = jest.fn();
});

describe("Search action output", () => {
	test("It gets log file path", () => {
		const task = getTask();
		const res = getLogFilePath(cwdStub, task);
		expect(res).toBe(`${cwdStub}/logs/example-up-cego.log`);
	});

	test("It stashes logs to file", async () => {
		const task = getTask();
		fs.ensureFileSync = jest.fn();
		fs.writeFileSync = jest.fn();
		await stashLogsToFile([task], cnfStub, "up");

		expect(fs.ensureFileSync).toHaveBeenCalledWith(`${cwdStub}/logs/example-up-cego.log`);
		expect(fs.writeFileSync).toHaveBeenCalledWith(`${cwdStub}/logs/example-up-cego.log`, expect.any(String));
	});

	test("It sort tasks by time then state", () => {
		const c = getTask();
		const a = getTask();
		assert(a.result);
		a.result.finishTime = new Date(0);
		const b = getTask();
		assert(b.result);
		b.result.finishTime = new Date(2);
		c.state = TaskState.SKIPPED_FAILED_DEPENDENCY;
		c.result = null;

		// a was finished before b.
		const ab = sortTasksByTimeThenState(a, b);
		expect(ab).toBe(-2);

		// first have firstStates
		const ac = sortTasksByTimeThenState(a, c);
		expect(ac).toBe(-1);

		// second have firstStates
		const ca = sortTasksByTimeThenState(c, a);
		expect(ca).toBe(1);
	});
	// test("It searches stdout", async () => {
	// 	const searchFor: SearchFor[] = [
	// 		{
	// 			// string contain any digit
	// 			regex: "[0-9]",
	// 			hint: "This string contains a digit",
	// 		},
	// 	];
	// 	const stdoutHistory: (GroupKey & ChildProcessOutput)[] = [
	// 		{
	// 			project: "project1",
	// 			action: "action1",
	// 			group: "group1",
	// 			stdout: "This string does contain 1 digit",
	// 			stderr: "This string does not contain a digit",
	// 		},
	// 	];

	// 	searchOutputForHints({ projects: {}, searchFor } as unknown as Config, stdoutHistory, false);

	// 	expect(console.log).toHaveBeenCalledTimes(1);
	// 	expect(console.log).toHaveBeenCalledWith(chalk`${searchFor[0].hint} {gray (Source: project1)}`);
	// });

	// test("It searches stderr", async () => {
	// 	const searchFor: SearchFor[] = [
	// 		{
	// 			// string contain any digit
	// 			regex: "[0-9]",
	// 			hint: "This string contains a digit",
	// 		},
	// 	];
	// 	const stdoutHistory: (GroupKey & ChildProcessOutput)[] = [
	// 		{
	// 			project: "project1",
	// 			action: "action1",
	// 			group: "group1",
	// 			stdout: "This string does contain 1 digit",
	// 			stderr: "This string does not contain a digit",
	// 		},
	// 	];

	// 	searchOutputForHints({ projects: {}, searchFor } as unknown as Config, stdoutHistory, false);

	// 	expect(console.log).toHaveBeenCalledTimes(1);
	// 	expect(console.log).toHaveBeenCalledWith(chalk`${searchFor[0].hint} {gray (Source: project1)}`);
	// });

	// test("It searches stdout and stderr", async () => {
	// 	const searchFor: SearchFor[] = [
	// 		{
	// 			// string contain any digit
	// 			regex: "[0-9]",
	// 			hint: "This string contains a digit",
	// 		},
	// 	];
	// 	const stdoutHistory: (GroupKey & ChildProcessOutput)[] = [
	// 		{
	// 			project: "project1",
	// 			action: "action1",
	// 			group: "group1",
	// 			stdout: "This string does contain 1 digit",
	// 			stderr: "This string does contain 1 digit",
	// 		},
	// 	];

	// 	searchOutputForHints({ projects: {}, searchFor } as unknown as Config, stdoutHistory, false);

	// 	expect(console.log).toHaveBeenCalledTimes(1);
	// 	expect(console.log).toHaveBeenCalledWith(chalk`${searchFor[0].hint} {gray (Source: project1)}`);
	// });

	// test("It searches multiple outputs", async () => {
	// 	const searchFor: SearchFor[] = [
	// 		{
	// 			// string contain any digit
	// 			regex: "[0-9]",
	// 			hint: "This string contains a digit",
	// 		},
	// 		{
	// 			// string contain any letter
	// 			regex: "[a-z]",
	// 			hint: "This string contains a letter",
	// 		},
	// 	];
	// 	const stdoutHistory: (GroupKey & ChildProcessOutput)[] = [
	// 		{
	// 			project: "project1",
	// 			action: "action1",
	// 			group: "group1",
	// 			stdout: "1",
	// 			stderr: "",
	// 		},
	// 		{
	// 			project: "project1",
	// 			action: "action1",
	// 			group: "group2",
	// 			stdout: "letter",
	// 			stderr: "",
	// 		},
	// 		{
	// 			project: "project1",
	// 			action: "action1",
	// 			group: "group3",
	// 			stdout: "@!???",
	// 			stderr: ":) 🤷‍♂️",
	// 		},
	// 		{
	// 			project: "project1",
	// 			action: "action1",
	// 			group: "group4",
	// 			stdout: "letter 123",
	// 			stderr: "",
	// 		},
	// 	];

	// 	searchOutputForHints({ projects: {}, searchFor } as unknown as Config, stdoutHistory, false);

	// 	expect(console.log).toHaveBeenCalledTimes(4);
	// 	expect(console.log).toHaveBeenCalledWith(chalk`${searchFor[0].hint} {gray (Source: project1)}`);
	// 	expect(console.log).toHaveBeenCalledWith(chalk`${searchFor[1].hint} {gray (Source: project1)}`);
	// 	expect(console.log).toHaveBeenCalledWith(chalk`${searchFor[0].hint} {gray (Source: project1)}`);
	// 	expect(console.log).toHaveBeenCalledWith(chalk`${searchFor[1].hint} {gray (Source: project1)}`);
	// });

	// test("It searches action specific searchFor", () => {
	// 	const searchFor: SearchFor[] = [
	// 		{
	// 			// string contain any digit
	// 			regex: "[0-9]",
	// 			hint: "This string contains a digit",
	// 		},
	// 	];
	// 	const stdoutHistory: (GroupKey & ChildProcessOutput)[] = [
	// 		{
	// 			project: "projecta",
	// 			action: "start",
	// 			group: "group1",
	// 			stdout: "This string does contain 1 digit",
	// 			stderr: "This string does not contain a digit",
	// 		},
	// 	];

	// 	const cfg: Config = JSON.parse(JSON.stringify(cnfStub));
	// 	cfg.projects["projecta"].actions["start"].searchFor = searchFor;

	// 	searchOutputForHints(cfg, stdoutHistory, false);

	// 	expect(console.log).toHaveBeenCalledTimes(1);
	// 	expect(console.log).toHaveBeenCalledWith(chalk`${searchFor[0].hint} {gray (Source: projecta)}`);
	// });

	// test("It supports chalking in searchFor", () => {
	// 	const searchFor: SearchFor[] = [
	// 		{
	// 			// string contain any digit
	// 			regex: "[0-9]",
	// 			hint: "{green This string contains a digit}",
	// 		},
	// 	];
	// 	const stdoutHistory: (GroupKey & ChildProcessOutput)[] = [
	// 		{
	// 			project: "projecta",
	// 			action: "start",
	// 			group: "group1",
	// 			stdout: "This string does contain 1 digit",
	// 			stderr: "This string does not contain a digit",
	// 		},
	// 	];

	// 	const cfg: Config = JSON.parse(JSON.stringify(cnfStub));
	// 	cfg.projects["projecta"].actions["start"].searchFor = searchFor;

	// 	searchOutputForHints(cfg, stdoutHistory, false);

	// 	expect(console.log).toHaveBeenCalledWith(chalk`{green This string contains a digit} {gray (Source: projecta)}`);
	// });
});
