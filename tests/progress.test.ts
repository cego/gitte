import {
	applyPromiseToEntriesWithProgressBar,
	applyPromiseToEntriesWithProgressBarSync,
	waitingOnToString,
} from "../src/progress";

describe("Progress helper", () => {
	describe("WaitingOnToString", () => {
		test("Finished all tasks", () => {
			expect(waitingOnToString([])).toBe("Finished all tasks");
		});
		test("One task", () => {
			expect(waitingOnToString(["test"])).toBe("test");
		});
		test("Two tasks", () => {
			expect(waitingOnToString(["test", "test2"])).toBe("test, test2");
		});
		test("Many tasks", () => {
			const waitingOn = [
				"long test string",
				"boring",
				"tests",
				"pls longer strings lmao",
				"aadw wd w d w d wd w verrryy long name",
			];
			expect(waitingOnToString(waitingOn)).toBe("long test string, boring, tests, pls longer strings lmao and 1 more");
		});
	});
	describe("Apply promise sync", () => {
		test("One task", async () => {
			const fn = jest.fn(() => Promise.resolve("test"));
			const result = await applyPromiseToEntriesWithProgressBarSync("test", [["test", "test"]], fn);
			expect(result).toEqual(["test"]);
			expect(fn).toHaveBeenCalledTimes(1);
		});

		test("Many tasks", async () => {
			const fn = jest.fn(() => Promise.resolve("test"));
			const result = await applyPromiseToEntriesWithProgressBarSync(
				"test",
				[
					["test", "test"],
					["test2", "test2"],
					["test3", "test3"],
					["test4", "test4"],
				],
				fn,
			);
			expect(result).toEqual(["test", "test", "test", "test"]);
			expect(fn).toHaveBeenCalledTimes(4);
		});

		test("One task succeed, one fail", async () => {
			const fn = jest.fn(() => Promise.reject("test"));

			await expect(applyPromiseToEntriesWithProgressBarSync("test", [["test", "test"]], fn)).rejects.toBe("test");
		});
	});

	describe("Apply promise async", () => {
		test("One task", async () => {
			const fn = jest.fn(() => Promise.resolve("test"));
			const result = await applyPromiseToEntriesWithProgressBar("test", [["test", "test"]], fn);
			expect(result).toEqual(["test"]);
			expect(fn).toHaveBeenCalledTimes(1);
		});

		test("Many tasks", async () => {
			const fn = jest.fn(() => Promise.resolve("test"));
			const result = await applyPromiseToEntriesWithProgressBar(
				"test",
				[
					["test", "test"],
					["test2", "test2"],
					["test3", "test3"],
					["test4", "test4"],
				],
				fn,
			);
			expect(result).toEqual(["test", "test", "test", "test"]);
			expect(fn).toHaveBeenCalledTimes(4);
		});

		test("One task succeed, one fail", async () => {
			const fn = jest.fn().mockImplementation(async (arg) => {
				if (arg === "test") return "test";
				else throw new Error("test");
			});
			const result = await applyPromiseToEntriesWithProgressBar(
				"test",
				[
					["test", "test"],
					["test2", "test2"],
				],
				fn,
			);
			expect(result).toEqual(["test", new Error("test")]);
			expect(fn).toHaveBeenCalledTimes(2);
		});
	});
});
