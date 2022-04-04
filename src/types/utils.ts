import * as pcp from "promisify-child-process";

export type ToChildProcessOutput = [(Error & pcp.Output) | null, pcp.Output | undefined];

export type GroupKey = {
	project: string;
	action: string;
	group: string;
};

export class ErrorWithHint extends Error {
	hint: string;
	constructor(hint: string, message = hint) {
		super(message);
		this.hint = hint;
	}
}
