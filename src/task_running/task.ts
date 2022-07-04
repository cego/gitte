import { GroupKey } from "../types/utils";
import * as utils from "../utils";
import { ExecaError, ExecaReturnValue } from "execa";
import { TaskHandler } from "./task_handler";
import { Writable } from "stream";

type OutType = "stdout" | "stderr";

type OutObject = {
	text: string;
	type: OutType;
}

type ActionResult = {
	out: OutObject[]
	exitCode: number;
	signal?: string;
	finishTime: Date;
};

type ActionContext = {
	cwd: string;
	cmd: string[];
	priority: number;
};

enum TaskState {
	PENDING = "pending",
	BLOCKED = "blocked",
	RUNNING = "running",
	COMPLETED = "completed",
	FAILED = "failed",
	SKIPPED_FAILED_DEPENDENCY = "skipped_failed_dependency",
}

class Task {
	constructor(public key: GroupKey, public context: ActionContext, public needs: GroupKey[]) {
		if (needs.length > 0) {
			this.state = TaskState.BLOCKED;
		}
	}

	public skippedBy?: Task;

	// tostring method
	public toString(): string {
		return `${this.key.project}/${this.key.action}/${this.key.group}`;
	}

	public state: TaskState = TaskState.PENDING;

	public result: ActionResult | null = null;

	public async run(printer: TaskHandler): Promise<void> {
		const promise = utils.spawn(this.context.cmd[0], this.context.cmd.slice(1), {
			cwd: this.context.cwd,
			env: process.env,
			encoding: "utf8",
			maxBuffer: 1024 * 2048,
		});

		promise.stdout?.pipe(printer.getWritableStream(this));
		promise.stderr?.pipe(printer.getWritableStream(this));

		// Also pipe stdout to save in task
		const out: OutObject[] = [];
		promise.stdout?.pipe(this.getWritableStream("stdout", out));
		promise.stderr?.pipe(this.getWritableStream("stderr", out));

		const res: ExecaReturnValue<string> | ExecaError<string> = await promise.catch((err) => err);

		this.result = {
			out,
			exitCode: res.exitCode,
			signal: res.signal,
			finishTime: new Date(),
		};
		this.state = TaskState.COMPLETED;
	}

	private getWritableStream(type: OutType, outArr: OutObject[]): Writable{
		return new Writable({
			write(chunk, _, callback) {
				const text: string[] = chunk.toString().split("\n");
				text.forEach(x => outArr.push({ text: x, type }));
				callback();
			},
		});
	}
}

export { Task, ActionResult, ActionContext, TaskState };
