import { GroupKey } from "../types/utils";
import * as utils from "../utils";
import { ExecaError, ExecaReturnValue } from "execa";
import _ from "lodash";
import { TaskHandler } from "./task_handler";

type ActionResult = {
    stdout: string;
    stderr: string;
    exitCode: number;
    signal?: string;
}

type ActionContext = {
    cwd: string;
    cmd: string[];
    priority: number;
}

enum TaskState {
    PENDING = "pending",
    BLOCKED = "blocked",
    RUNNING = "running",
    COMPLETED = "completed",
    FAILED = "failed",
    SKIPPED_DUPLICATE = "skipped_duplicate",
    SKIPPED_FAILED_DEPENDENCY = "skipped_failed_dependency",
}


class Task {
    constructor(
        public key: GroupKey,
        public context: ActionContext,
        public needs: GroupKey[]
    ) { }

    public state: TaskState = TaskState.PENDING;

    public result: ActionResult | null = null;

    public async run(printer: TaskHandler): Promise<void> {
        const promise = utils.spawn(this.context.cmd[0], this.context.cmd.slice(1), {
            cwd: this.context.cwd,
            env: process.env,
            encoding: "utf8",
            maxBuffer: 1024 * 2048,
        });

        promise.stdout?.pipe(printer.getWritableStream(this.key.project));
        promise.stderr?.pipe(printer.getWritableStream(this.key.project));

        const res: ExecaReturnValue<string> | ExecaError<string> = await promise.catch((err) => err);

        this.result = {
            stdout: res.stdout?.toString() ?? "",
            stderr: res.stderr?.toString() ?? "",
            exitCode: res.exitCode,
            signal: res.signal,
        };
    }
}

export { Task, ActionResult, ActionContext, TaskState };