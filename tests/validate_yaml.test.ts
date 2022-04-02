import { Config } from "../src/types/config";
import { validateYaml } from "../src/validate_yaml";
import { cnfStub } from "./utils/stubs";

describe("Test validator", () => {
	let cnf: any;
	beforeEach(() => {
		cnf = { ...cnfStub };
		delete cnf.cwd;
		console.log = jest.fn();
	});

	test("Returns valid object", () => {
		const res = validateYaml(cnf);
		expect(res).toBeDefined();
	});

	test("Logs error and returns falsy for invalid object", () => {
		console.error = jest.fn();
		const res = validateYaml({});
		expect(res).toBeFalsy();
		expect(console.error).toHaveBeenCalled();
	});
});
