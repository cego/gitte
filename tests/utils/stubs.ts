import { Task, TaskState } from "../../src/task_running/task";
import { CmdAction, ShellAction } from "../../src/types/config";

export const projectStub: any = {
	default_branch: "main",
	remote: "git@gitlab.com:cego/example.git",
	actions: {
		start: {
			searchFor: [],
			needs: [],
			groups: {
				"cego.dk": ["docker-compose", "up"],
				"example.com": ["scp", "user@example.com", "sh", "-c", "service", "webserver", "start"],
			},
		},
		down: {
			searchFor: [],
			needs: [],
			groups: {
				"cego.dk": ["docker-compose", "down"],
				"example.com": ["scp", "user@example.com", "sh", "-c", "service", "webserver", "stop"],
			},
		},
	},
};
export const startupStub: { [key: string]: CmdAction | ShellAction } = {
	world: { cmd: ["echo", "world"] },
	bashWorld: { shell: "bash", script: "echo world" },
};
export const cnfStub: any = {
	cwd: "/home/user/gitte",
	startup: startupStub,
	projects: {
		projecta: projectStub,
		projectd: {
			default_branch: "main",
			remote: "git@gitlab.com:cego/exampled.git",
			actions: {
				up: {
					searchFor: [],
					needs: ["projecte"],
					groups: {
						"cego.dk": ["docker-compose", "up"],
					},
				},
			},
		},
		projecte: {
			default_branch: "main",
			remote: "git@gitlab.com:cego/exampled.git",
			actions: {
				up: {
					searchFor: [],
					needs: [],
					groups: {
						"cego.dk": ["docker-compose", "up"],
					},
				},
			},
		},
		disabledProject: {
			defaultDisabled: true,
			default_branch: "main",
			remote: "git@gitlab.com:cego/disabled-project.git",
			actions: {},
		},
	},
	searchFor: [],
};
export const cwdStub = "/home/user/gitte";

export const getTask = (): Task => {
	const task = new Task(
		{ project: "example", action: "up", group: "cego" },
		{ cwd: cwdStub, cmd: ["woot", "a"], priority: 0 },
		[],
	);

	task.result = {
		out: ["Hello World 123"],
		exitCode: 0,
		finishTime: new Date(),
	};

	task.state = TaskState.COMPLETED;

	return task;
};
