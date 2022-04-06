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
};

export class ErrorWithHint extends Error {
	hint: string;
	constructor(hint: string, message = hint) {
		super(message);
		this.hint = hint;
	}
}
