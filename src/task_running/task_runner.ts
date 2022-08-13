import { Task, TaskState } from "./task";
import assert from "assert";
import { TaskHandler } from "./task_handler";
import { compareGroupKeys } from "../utils";

/**
 * Class that, given a list of tasks, will run them.
 * Should respect the dependencies of the tasks.
 * Should respect the priority of the tasks.
 * Should skip tasks which a dependency failed
 *
 * This class is NOT responsible for constructing the tasks but simply running them respecting above rules.
 */
class TaskRunner {
	public tasks: Task[];
	private taskQueue: Task[] = [];
	constructor(
		tasksIn: Task[],
		private actionOutputPrinter: TaskHandler,
		action: string,
		private maxTaskParallelization: number,
	) {
		this.tasks = tasksIn.filter((task) => task.key.action == action);
	}

	public async run(): Promise<void> {
		const priorities = this.getUniquePriorities();
		for (const priority of priorities) {
			const beginningTasks = this.tasks.filter(
				(task) => task.context.priority === priority && task.needs.length === 0 && task.state === TaskState.PENDING,
			);

			// Take maxTaskParallelization number of tasks from beginningTasks and put the rest in taskQueue
			const tasksToRun = beginningTasks.slice(0, this.maxTaskParallelization);
			this.taskQueue = beginningTasks.slice(this.maxTaskParallelization);

			const promises = tasksToRun.map((task) => this.wrapTask(task));
			await Promise.all(promises);
		}
	}

	private wrapTask(taskToRun: Task): Promise<void> {
		this.actionOutputPrinter.beganTask(taskToRun);
		return taskToRun.run(this.actionOutputPrinter).then(async () => {
			this.actionOutputPrinter.finishedTask(taskToRun);

			assert(taskToRun.result);

			if (taskToRun.result.exitCode !== 0) {
				this.skipAllBlockedActions(taskToRun);
				taskToRun.state = TaskState.FAILED;
				await this.runNextTask();
				return;
			}

			const taskFreed = this.tasks
				.filter((task) => task.state === TaskState.BLOCKED)
				.reduce((carry, task) => {
					task.needs = task.needs.filter((need) => !compareGroupKeys(need, taskToRun.key));
					if (task.needs.length === 0) {
						task.state = TaskState.PENDING;
						return [...carry, task];
					}
					return [...carry];
				}, [] as Task[]);

			// Add tasks that are freed to the taskQueue
			this.taskQueue = [...this.taskQueue, ...taskFreed];
			await this.runNextTask();
		});
	}

	skipAllBlockedActions(taskIn: Task) {
		taskIn.state = TaskState.SKIPPED_FAILED_DEPENDENCY;
		this.tasks
			.filter((task) => task.state === TaskState.BLOCKED)
			.filter(
				(task) =>
					task.needs.filter(
						(needKey) =>
							needKey.action == taskIn.key.action &&
							needKey.group == taskIn.key.group &&
							needKey.project == taskIn.key.project,
					).length > 0,
			)
			.forEach((task) => {
				task.skippedBy = taskIn;
				this.skipAllBlockedActions(task);
			});
	}

	private getUniquePriorities(): number[] {
		const priorities = this.tasks.map((task) => task.context.priority);
		return [...new Set(priorities)];
	}

	private async runNextTask(): Promise<void> {
		if (this.taskQueue.length === 0) {
			return;
		}
		const nextTask = this.taskQueue.shift();
		if (nextTask) {
			await this.wrapTask(nextTask);
		}
	}
}

export { TaskRunner };
