import { validateYaml } from "../src/validate_yaml";
import { cnfStub } from "./utils/stubs";

describe("Test validator", () => {
	console.log = jest.fn();

	test("Returns valid object", () => {
		const res = validateYaml(cnfStub);
		expect(res).toBeDefined();
	});

	test("Logs error and returns falsy for invalid object", () => {
		console.error = jest.fn();
		const res = validateYaml({});
		expect(res).toBeFalsy();
		expect(console.error).toHaveBeenCalled();
	});
});
