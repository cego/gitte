import { GroupKey } from "../types/utils";
import * as utils from "../utils";
import { ExecaError, ExecaReturnValue } from "execa";
import { TaskHandler } from "./task_handler";

type ActionResult = {
	stdout: string;
	stderr: string;
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

		const res: ExecaReturnValue<string> | ExecaError<string> = await promise.catch((err) => err);

		this.result = {
			stdout: res.stdout?.toString() ?? "",
			stderr: res.stderr?.toString() ?? "",
			exitCode: res.exitCode,
			signal: res.signal,
			finishTime: new Date(),
		};
		this.state = TaskState.COMPLETED;
	}
}

export { Task, ActionResult, ActionContext, TaskState };
