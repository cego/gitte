import Ajv2019 from "ajv/dist/2019";
import { createActionGraphs } from "./graph";
import { Config } from "./types/config";

const ajv = new Ajv2019();

const schema = {
	type: "object",
	required: ["startup", "projects"],
	additionalProperties: false,
	properties: {
		searchFor: {
			type: "array",
			items: {
				type: "object",
				required: ["regex", "hint"],
				additionalProperties: false,
				properties: {
					regex: {
						type: "string",
					},
					hint: {
						type: "string",
					},
				},
			},
		},
		startup: {
			type: "object",
			additionalProperties: {
				anyOf: [
					{
						type: "object",
						required: ["cmd"],
						additionalProperties: false,
						properties: {
							cmd: {
								type: "array",
								contains: { type: "string" },
								minContains: 1,
							},
							hint: {
								type: "string",
							},
						},
					},
					{
						type: "object",
						required: ["shell", "script"],
						additionalProperties: false,
						properties: {
							shell: {
								type: "string",
							},
							script: {
								type: "string",
							},
							hint: {
								type: "string",
							},
						},
					},
				],
			},
		},
		projects: {
			type: "object",
			additionalProperties: {
				type: "object",
				required: ["remote", "default_branch"],
				additionalProperties: false,
				properties: {
					remote: {
						type: "string",
					},
					default_branch: {
						type: "string",
					},
					actions: {
						type: "object",
						additionalProperties: {
							type: "object",
							additionalProperties: false,
							properties: {
								priority: {
									type: "integer",
								},
								needs: {
									type: "array",
									items: {
										type: "string",
									},
								},
								groups: {
									type: "object",
									additionalProperties: {
										type: "array",
										contains: { type: "string" },
									},
								},
								searchFor: {
									type: "array",
									items: {
										type: "object",
										required: ["regex", "hint"],
										additionalProperties: false,
										properties: {
											regex: {
												type: "string",
											},
											hint: {
												type: "string",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	},
};

const validate = ajv.compile<Config>(schema);

export function validateYaml(obj: any): obj is Config {
	const valid = validate(obj);
	if (!valid) {
		console.error(validate.errors);
		return false;
	}

	// Fail fast if graph is invalid.
	try {
		createActionGraphs(obj);
	} catch (e: any) {
		console.log(e.message);
		return false;
	}

	return true;
}
