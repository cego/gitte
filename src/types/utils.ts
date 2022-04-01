import * as pcp from "promisify-child-process";

export type ToChildProcessOutput = [(Error & pcp.Output) | null, pcp.Output | undefined];

export type GroupKey = {
	project: string;
	action: string;
	group: string;
};

export type ActionOutput = GroupKey & pcp.Output & { dir: string; cmd: string[] };
