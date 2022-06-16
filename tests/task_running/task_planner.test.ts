import { AssertionError } from "assert";
import { TaskPlanner } from "../../src/task_running/task_planner";
import { cnfStub } from "../utils/stubs";

describe("Task Planner tests", () => {
	describe("planStringInput", () => {
		it("should rewrite 'all' to '*'", () => {
			const actions = "all";
			const groups = "all";
			const projects = "all";

			const plan = new TaskPlanner(cnfStub).planStringInput(actions, groups, projects);

			expect(plan).toHaveLength(6);
		});

		it("should error if no tasks are found", () => {
			const actions = "";
			const groups = "";
			const projects = "";

			// should throw AssertionError
			expect(() => {
				new TaskPlanner(cnfStub).planStringInput(actions, groups, projects);
			}).toThrowError(AssertionError);
		});
	});

	describe("resolveNeeds", () => {
		it("should resolve needs", () => {
			const keySet = { action: "a", group: "b", project: "c" };
			const needs = ["f"];
			const keySets = [
				{ action: "a", group: "b", project: "f" },
				{ action: "a", group: "b", project: "i" },
			];

			const resolved = new TaskPlanner(cnfStub).resolveNeeds(keySet, needs, keySets);

			expect(resolved).toHaveLength(1);
			expect(resolved[0]).toEqual({ action: "a", group: "b", project: "f" });
		});

		it("should fallback to * group if specific group dont exist", () => {
			const keySet = { action: "a", group: "b", project: "c" };
			const needs = ["f"];
			const keySets = [
				{ action: "a", group: "*", project: "f" },
				{ action: "a", group: "b", project: "i" },
			];

			const resolved = new TaskPlanner(cnfStub).resolveNeeds(keySet, needs, keySets);

			expect(resolved).toHaveLength(1);
			expect(resolved[0]).toEqual({ action: "a", group: "*", project: "f" });
		});
	});
});
