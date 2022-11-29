import { Task, TaskState } from "./task";
import assert from "assert";
import { TaskHandler } from "./task_handler";
import { compareGroupKeys } from "../utils";
import { Config } from "../types/config";

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

	constructor(
		tasksIn: Task[],
		private actionOutputPrinter: TaskHandler,
		action: string,
		public maxTaskParallelization: number,
		private config: Config,
	) {
		this.tasks = tasksIn.filter((task) => task.key.action == action);

		if (this.config.actionOverride && Object.keys(this.config.actionOverride).includes(action)) {
			const actionOverride = this.config.actionOverride[action];
			if (actionOverride.maxParallelization) {
				this.maxTaskParallelization = actionOverride.maxParallelization;
			}
		}
	}

	public async run(): Promise<void> {
		const priorities = this.getUniquePriorities();
		for (const priority of priorities) {
			const beginningTasks = this.tasks.filter(
				(task) => task.context.priority === priority && task.needs.length === 0 && task.state === TaskState.PENDING,
			);

			// Take maxTaskParallelization number of tasks from beginningTasks and put the rest in taskQueue
			const tasksToRun = beginningTasks.slice(0, this.maxTaskParallelization);

			// Mark the rest of the beginningTasks as QUEUED
			beginningTasks.slice(this.maxTaskParallelization).forEach((task) => {
				task.state = TaskState.QUEUED;
			});

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

			this.tasks
				.filter((task) => task.state === TaskState.BLOCKED)
				.reduce((carry, task) => {
					task.needs = task.needs.filter((need) => !compareGroupKeys(need, taskToRun.key));
					if (task.needs.length === 0) {
						task.state = TaskState.PENDING;
						return [...carry, task];
					}
					return [...carry];
				}, [] as Task[])
				.forEach((task) => (task.state = TaskState.QUEUED));

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
		// If there no more tasks to run, return
		if (this.tasks.filter((task) => task.state === TaskState.QUEUED).length === 0) {
			return;
		}

		// Find out how many more tasks we can start by looking at how many tasks are currently running
		const tasksRunning = this.tasks.filter((task) => task.state === TaskState.RUNNING).length;
		const tasksToStart = Math.max(this.maxTaskParallelization - tasksRunning, 1);

		// Get the next tasks to run
		const tasksToRun = this.tasks
			.filter((task) => task.state === TaskState.QUEUED)
			.slice(0, tasksToStart)
			.map((task) => {
				return this.wrapTask(task);
			});

		await Promise.all(tasksToRun);
	}
}

export { TaskRunner };
