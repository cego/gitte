import { ExecaReturnValue } from "execa";

export type ToChildProcessOutput = [(Error & ExecaReturnValue) | null, ExecaReturnValue | undefined];

export type GroupKey = {
	project: string;
	action: string;
	group: string;
};

export type ChildProcessOutput = {
	stdout?: string;
	stderr?: string;
	exitCode?: number;
	signal?: string;
	cmd?: string[];
	dir?: string;
};

export class ErrorWithHint extends Error {
	hint: string;
	constructor(hint: string, error = new Error()) {
		super(error.message);
		Object.assign(this, error);
		this.hint = hint;
	}
}
