import Ajv2019 from "ajv/dist/2019";
import { ActionGraphs, createActionGraphs } from "./graph";
import { Config } from "./types/config";

const ajv = new Ajv2019();

const schema = {
	type: "object",
	required: ["startup", "projects"],
	properties: {
		searchFor: {
			type: "array",
			items: {
				type: "object",
				required: ["regex", "hint"],
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
				properties: {
					remote: {
						type: "string",
					},
					default_branch: {
						type: "string",
					},
					priority: {
						type: "integer",
						minimum: 0,
						maximum: 1000,
					},
					actions: {
						type: "object",
						additionalProperties: {
							type: "object",
							properties: {
								priority: {
									type: "integer",
									minimum: 0,
									maximum: 1000,
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
										minContains: 1,
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

export function validateYaml(obj: any): Config & ActionGraphs {
	const valid = validate(obj);
	if (!valid) {
		console.error(validate.errors);
	}

	const actionGraphs = createActionGraphs(obj);

	return { ...obj, actionGraphs };
}
