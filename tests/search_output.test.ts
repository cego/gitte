import {
	getLogFilePath,
	logTaskOutput,
	searchOutputForHints,
	sortTasksByTimeThenState,
	stashLogsToFile,
} from "../src/search_output";
import { cnfStub, cwdStub, getTask } from "./utils/stubs";
import fs from "fs-extra";
import { TaskState } from "../src/task_running/task";
import assert from "assert";
import chalk from "chalk";
import _ from "lodash";
import { Config } from "../src/types/config";

beforeEach(() => {
	console.log = jest.fn();
	console.error = jest.fn();
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

	test("It logs task output success", async () => {
		const task = getTask();

		const error = await logTaskOutput([task], cwdStub, "up");
		expect(error).toBe(false);

		expect(console.log).toHaveBeenCalledWith(
			chalk`{bgGreen  PASS } {bold example/up/cego} {blue woot a} ran in {cyan /home/user/gitte}`,
		);
	});

	test("It logs task output skipped", async () => {
		const task = getTask();
		task.skippedBy = task;
		task.state = TaskState.SKIPPED_FAILED_DEPENDENCY;

		const error = await logTaskOutput([task], cwdStub, "up");
		expect(error).toBe(false);

		expect(console.error).toHaveBeenCalledWith(expect.stringContaining("because it needed"));
	});

	test("It logs task output failed", async () => {
		const task = getTask();
		task.state = TaskState.FAILED;

		const error = await logTaskOutput([task], cwdStub, "up");
		expect(error).toBe(true);

		expect(console.error).toHaveBeenCalledWith(expect.stringContaining("FAIL"));
	});

	test("It searched output for hints global", async () => {
		const task = getTask();
		const config: Config = _.cloneDeep(cnfStub);
		config.searchFor = [
			{
				// string contain any digit
				regex: "[0-9]",
				hint: "This string contains a digit",
			},
		];

		searchOutputForHints([task], config, "up");

		expect(console.log).toHaveBeenCalledTimes(4);
		expect(console.log).toHaveBeenCalledWith(expect.stringContaining("This string contains a digit"));
	});

	test("It searched output for hints local", async () => {
		const task = getTask();
		const config: Config = _.cloneDeep(cnfStub);
		config.projects[task.key.project].actions[task.key.action].searchFor = [
			{
				// string contain any digit
				regex: "[a-z]",
				hint: "This string contains a letter",
			},
		];

		searchOutputForHints([task], config, "up");

		expect(console.log).toHaveBeenCalledTimes(4);
		expect(console.log).toHaveBeenCalledWith(expect.stringContaining("This string contains a letter"));
	});
});
