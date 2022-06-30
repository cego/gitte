import { AssertionError } from "assert";
import _ from "lodash";
import { GroupKeyWithDependencies, TaskPlanner } from "../../src/task_running/task_planner";
import { Config } from "../../src/types/config";
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

	describe("addProjectDependencies and removeUnrunnable", () => {
		it("should add dependencies and remove unrunnable", () => {
			const config: Config = _.cloneDeep(cnfStub);

			config.projects = {
				service: {
					remote: "",
					default_branch: "",
					actions: {
						a: {
							searchFor: [],
							priority: 0, // not relevant for test
							needs: ["terraform"],
							groups: {
								b: ["echo", "b"],
							},
						},
					},
				},
				terraform: {
					remote: "",
					default_branch: "",
					actions: {
						a: {
							searchFor: [],
							priority: 0, // not relevant for test
							needs: ["mysql"],
							groups: {
								b: ["echo", "b"],
							},
						},
					},
				},
				mysql: {
					remote: "",
					default_branch: "",
					actions: {
						a: {
							searchFor: [],
							priority: 0, // not relevant for test
							needs: ["bootOs"],
							groups: {
								b: ["echo", "b"],
							},
						},
					},
				},
				bootOs: {
					remote: "",
					default_branch: "",
					actions: {
						a: {
							searchFor: [],
							priority: 0, // not relevant for test
							needs: ["nginx"],
							groups: {
								c: ["echo", "b"],
							},
						},
					},
				},
				nginx: {
					remote: "",
					default_branch: "",
					actions: {
						a: {
							searchFor: [],
							priority: 0, // not relevant for test
							needs: [],
							groups: {
								"*": ["echo", "b"],
							},
						},
					},
				},
			};

			const keySets: GroupKeyWithDependencies[] = [{ action: "a", group: "b", project: "service", needs: [] }];

			const planner = new TaskPlanner(config);
			const resolved = planner.addProjectDependencies(keySets);

			expect(resolved).toHaveLength(5);
			expect(resolved).toContainEqual({
				action: "a",
				group: "b",
				project: "service",
				needs: [{ action: "a", group: "b", project: "terraform", needs: [] }],
			});
			expect(resolved).toContainEqual({
				action: "a",
				group: "b",
				project: "terraform",
				needs: [{ action: "a", group: "b", project: "mysql", needs: [] }],
			});
			expect(resolved).toContainEqual({
				action: "a",
				group: "b",
				project: "mysql",
				needs: [{ action: "a", group: "!", project: "bootOs", needs: [] }],
			});
			expect(resolved).toContainEqual({
				action: "a",
				group: "!",
				project: "bootOs",
				needs: [{ action: "a", group: "*", project: "nginx", needs: [] }],
			});
			expect(resolved).toContainEqual({ action: "a", group: "*", project: "nginx", needs: [] });

			const final = planner.removeUnrunnable(resolved);
			expect(final).toHaveLength(4);
			expect(final).toContainEqual({
				action: "a",
				group: "b",
				project: "mysql",
				needs: [{ action: "a", group: "*", project: "nginx", needs: [] }],
			});
		});
	});
});
