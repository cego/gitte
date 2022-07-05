import Ajv2019 from "ajv/dist/2019";
import path from "path";
import { Config } from "./types/config";
import fs from "fs-extra";
import { configSchema } from "./validate_yaml";
import { AssertionError } from "assert";

const ajv = new Ajv2019();

export type Cache = {
	version: number;
	seenProjects: string[];
	config: Config;
};

const schema = {
	type: "object",
	required: ["version", "seenProjects", "config"],
	additionalProperties: true,
	properties: {
		version: {
			type: "number",
			minimum: 1,
			maximum: 1,
		},
		seenProjects: {
			type: "array",
			uniqueItems: true,
			items: {
				type: "string",
			},
		},
		config: configSchema,
	},
};

const validate = ajv.compile<Cache>(schema);

export function validateCache(cache: any): cache is Cache {
	const valid = validate(cache);
	if (!valid) {
		console.error(validate.errors);
		return false;
	}
	return true;
}

export function getCachePathFromCwd(cwd: string): string | null {
	const cachePath = path.join(cwd, ".gitte-cache.json");
	if (fs.pathExistsSync(cachePath)) {
		return cachePath;
	} else if (cwd === "/") {
		return null;
	} else {
		return getCachePathFromCwd(path.resolve(cwd, ".."));
	}
}

export function loadCachePath(cwd: string): Cache | null {
	const cachePath = getCachePathFromCwd(cwd);
	if (cachePath === null) {
		return null;
	}
	const cache = fs.readJSONSync(cachePath);
	if (!validateCache(cache)) {
		throw new AssertionError({ message: "Cache is invalid" });
	}
	return cache;
}

export function loadCacheCwd(cwd: string): Cache | null {
	const cachePath = getCachePathFromCwd(cwd);
	if (cachePath === null) {
		return null;
	}
	return loadCachePath(cachePath);
}
