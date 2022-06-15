import { Task, TaskState } from "./task";
import assert from "assert";
import { TaskHandler } from "./task_handler";
import { compareGroupKeys } from "../utils";

/**
 * Class that, given a list of tasks, will run them.
 * Should respect the dependencies of the tasks.
 * Should respect the priority of the tasks.
 * Should skip duplicate tasks - TODO maybe this should be handled by deduplicating before.
 * Should skip tasks which a dependency failed
 *
 * This class is NOT responsible for constructing the tasks but simply running them respecting above rules.
 */
class TaskRunner {
	public tasks: Task[];
	constructor(tasksIn: Task[], private actionOutputPrinter: TaskHandler, action: string) {
		this.tasks = tasksIn.filter((task) => task.key.action == action);
	}

	public async run(): Promise<void> {
		const priorities = this.getUniquePriorities();
		for (const priority of priorities) {
			const beginningTasks = this.tasks.filter(
				(task) => task.context.priority === priority && task.needs.length === 0 && task.state === TaskState.PENDING,
			);

			const promises = beginningTasks.map((task) => this.wrapTask(task));
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
				return;
			}

			const taskFreedPromises = this.tasks
				.filter((task) => task.state === TaskState.BLOCKED)
				.reduce((carry, task) => {
					task.needs = task.needs.filter((need) => !compareGroupKeys(need, taskToRun.key));
					if (task.needs.length === 0) {
						task.state = TaskState.PENDING;
						return [...carry, task];
					}
					return [...carry];
				}, [] as Task[])
				.map((task) => this.wrapTask(task));

			await Promise.all(taskFreedPromises);
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
				this.skipAllBlockedActions(task);
			});
	}

	private getUniquePriorities(): number[] {
		const priorities = this.tasks.map((task) => task.context.priority);
		return [...new Set(priorities)];
	}
}

export { TaskRunner };
