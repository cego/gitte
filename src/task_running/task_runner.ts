import { Task, TaskState } from "./task";
import { ActionOutputPrinter } from "../actions_utils";
import assert from "assert";

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

    constructor(
        private tasks: Task[],
        private actionOutputPrinter: ActionOutputPrinter
    ) { }

    public async run(): Promise<void> {
        const priorities = this.getUniquePriorities();
        for (const priority of priorities) {
            const beginningTasks = this.tasks.filter(
                (task) => task.context.priority === priority && task.needs.length === 0 && task.state === TaskState.PENDING);

            const promises = beginningTasks.map((task) => this.wrapTask(task));
            await Promise.all(promises);
        }
    }

    private wrapTask(taskToRun: Task): Promise<void> {
        return taskToRun.run(this.actionOutputPrinter).then(async () => {
            this.actionOutputPrinter.finishedTask(taskToRun);

            assert(taskToRun.result)

            if (taskToRun.result.exitCode !== 0) {
                this.skipAllBlockedActions(taskToRun);
                taskToRun.state = TaskState.FAILED;
                return;
            }

            const taskFreedPromises = this.tasks.filter((task) => task.state === TaskState.BLOCKED)
                .reduce(
                    (carry, task) => {
                        task.needs = task.needs.filter((need) => need === task.key);
                        if (task.needs.length === 0) {
                            task.state = TaskState.PENDING;
                            return [...carry, task]
                        }
                        return [...carry];
                    }, [] as Task[]
                ).map(
                    (task) => this.wrapTask(task)
                );

            Promise.all(taskFreedPromises);
        });
    }

    skipAllBlockedActions(taskIn: Task) {
        taskIn.state = TaskState.SKIPPED_FAILED_DEPENDENCY
        this.tasks.filter((task) => task.state === TaskState.BLOCKED)
            .filter((task) => task.needs.includes(taskIn.key))
            .forEach((task) => {
                this.skipAllBlockedActions(task);
            });
    }

    private getUniquePriorities(): number[] {
        const priorities = this.tasks.map((task) => task.context.priority);
        return [...new Set(priorities)];
    }
}