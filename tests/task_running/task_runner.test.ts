import assert from "assert";
import { TaskRunner } from "../../src/task_running/task_runner";
import { cnfStub } from "../utils/stubs";
import { TaskHandler } from "../../src/task_running/task_handler";
import _ from "lodash";
import { Config } from "../../src/types/config";

describe("Task Runner tests", () => {
	it("works", () => {
		assert(true);
	});
	it("overrides actionOverride maxParallelization", () => {
		const config: Config = _.cloneDeep(cnfStub);

		config.actionOverride = {
			action: {
				maxParallelization: 2,
			},
		};
		const taskRunner = new TaskRunner([], new TaskHandler(config, [], ["some-action"], 123), "action", 10, config);

		expect(taskRunner.maxTaskParallelization).toBe(2);
	});

	it("keeps actionOverride maxParallelization isolated", () => {
		const config: Config = _.cloneDeep(cnfStub);

		config.actionOverride = {
			"another-action": {
				maxParallelization: 2,
			},
		};
		const taskRunner = new TaskRunner([], new TaskHandler(config, [], ["some-action"], 123), "action", 10, config);

		expect(taskRunner.maxTaskParallelization).toBe(10);
	});
});
