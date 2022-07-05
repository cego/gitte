import Ajv2019 from "ajv/dist/2019";
import { createActionGraphs } from "./graph";
import { Config } from "./types/config";

const ajv = new Ajv2019();

export const configSchema = {
	type: "object",
	required: ["startup", "projects"],
	additionalProperties: true,
	properties: {
		searchFor: {
			type: "array",
			items: {
				type: "object",
				required: ["regex", "hint"],
				additionalProperties: true,
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
						additionalProperties: true,
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
						additionalProperties: true,
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
				additionalProperties: true,
				properties: {
					remote: {
						type: "string",
					},
					default_branch: {
						type: "string",
					},
					defaultDisabled: {
						type: "boolean",
					},
					actions: {
						type: "object",
						additionalProperties: {
							type: "object",
							additionalProperties: true,
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
										additionalProperties: true,
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

const validate = ajv.compile<Config>(configSchema);

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
